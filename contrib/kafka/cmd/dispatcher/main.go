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

	"github.com/knative/eventing/contrib/kafka/pkg/controller"
	"github.com/knative/eventing/contrib/kafka/pkg/dispatcher"
	"github.com/knative/eventing/contrib/kafka/pkg/utils"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/channelwatcher"
	topicUtils "github.com/knative/eventing/pkg/provisioners/utils"
	"github.com/knative/eventing/pkg/tracing"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func main() {
	flag.Parse()
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("unable to create logger: %v", err)
	}
	provisionerConfig, err := utils.GetKafkaConfig("/etc/config-provisioner")
	if err != nil {
		logger.Fatal("unable to load provisioner config", zap.Error(err))
	}

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		logger.Fatal("unable to create manager.", zap.Error(err))
	}

	args := &dispatcher.KafkaDispatcherArgs{
		ClientID:     fmt.Sprintf("%s-dispatcher", controller.Name),
		Brokers:      provisionerConfig.Brokers,
		ConsumerMode: provisionerConfig.ConsumerMode,
		TopicFunc:    topicUtils.TopicName,
		Logger:       logger,
	}
	kafkaDispatcher, err := dispatcher.NewDispatcher(args)
	if err != nil {
		logger.Fatal("unable to create kafka dispatcher.", zap.Error(err))
	}
	if err = mgr.Add(kafkaDispatcher); err != nil {
		logger.Fatal("Unable to add kafkaDispatcher", zap.Error(err))
	}

	if err := v1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		logger.Fatal("Unable to add scheme for eventing apis.", zap.Error(err))
	}

	// Zipkin tracing.
	kc := kubernetes.NewForConfigOrDie(mgr.GetConfig())
	configMapWatcher := configmap.NewInformedWatcher(kc, system.Namespace())
	if err = tracing.SetupDynamicZipkinPublishing(logger.Sugar(), configMapWatcher, "kafka-dispatcher"); err != nil {
		logger.Fatal("Error setting up Zipkin publishing", zap.Error(err))
	}

	if err = channelwatcher.New(mgr, logger, channelwatcher.UpdateConfigWatchHandler(kafkaDispatcher.UpdateConfig, shouldWatch)); err != nil {
		logger.Fatal("Unable to create channel watcher.", zap.Error(err))
	}

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	// configMapWatcher does not block, so start it first.
	if err = configMapWatcher.Start(stopCh); err != nil {
		logger.Fatal("Failed to start ConfigMap watcher", zap.Error(err))
	}

	// Start blocks forever.
	err = mgr.Start(stopCh)
	if err != nil {
		logger.Fatal("Manager.Start() returned an error", zap.Error(err))
	}
	logger.Info("Exiting...")
}

func shouldWatch(ch *v1alpha1.Channel) bool {
	return ch.Spec.Provisioner != nil &&
		ch.Spec.Provisioner.Namespace == "" &&
		ch.Spec.Provisioner.Name == controller.Name
}
