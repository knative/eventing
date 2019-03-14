/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	provisionerController "github.com/knative/eventing/contrib/kafka/pkg/controller"
	"github.com/knative/eventing/contrib/kafka/pkg/dispatcher"
	"github.com/knative/eventing/pkg/sidecar/configmap/watcher"
	"github.com/knative/eventing/pkg/utils"
	"github.com/knative/pkg/signals"
	"github.com/knative/pkg/system"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func main() {
	configMapName := os.Getenv("DISPATCHER_CONFIGMAP_NAME")
	if configMapName == "" {
		configMapName = provisionerController.DispatcherConfigMapName
	}
	configMapNamespace := os.Getenv("DISPATCHER_CONFIGMAP_NAMESPACE")
	if configMapNamespace == "" {
		configMapNamespace = system.Namespace()
	}

	flag.Parse()
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("unable to create logger: %v", err)
	}

	provisionerConfig, err := provisionerController.GetProvisionerConfig("/etc/config-provisioner")
	if err != nil {
		logger.Fatal("unable to load provisioner config", zap.Error(err))
	}

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		logger.Fatal("unable to create manager.", zap.Error(err))
	}

	kafkaDispatcher, err := dispatcher.NewDispatcher(provisionerConfig.Brokers, provisionerConfig.ConsumerMode, logger)
	if err != nil {
		logger.Fatal("unable to create kafka dispatcher.", zap.Error(err))
	}
	if err = mgr.Add(kafkaDispatcher); err != nil {
		logger.Fatal("Unable to add kafkaDispatcher", zap.Error(err))
	}

	kc, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		logger.Fatal("unable to create kubernetes client.", zap.Error(err))
	}

	cmw, err := watcher.NewWatcher(logger, kc, configMapNamespace, configMapName, kafkaDispatcher.UpdateConfig)
	if err != nil {
		logger.Fatal("unable to create configMap watcher", zap.String("configMap", fmt.Sprintf("%s/%s", configMapNamespace, configMapName)))
	}
	if err = mgr.Add(utils.NewBlockingStart(logger, cmw)); err != nil {
		logger.Fatal("Unable to add the configMap watcher to the manager", zap.Error(err))
	}

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()
	err = mgr.Start(stopCh)
	if err != nil {
		logger.Fatal("Manager.Start() returned an error", zap.Error(err))
	}
	logger.Info("Exiting...")
}
