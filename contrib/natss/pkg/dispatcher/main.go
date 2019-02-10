/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"log"
	"time"

	"github.com/knative/eventing/contrib/natss/pkg/dispatcher/channel"
	"github.com/knative/eventing/contrib/natss/pkg/dispatcher/dispatcher"
	"github.com/knative/pkg/signals"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/knative/eventing/contrib/natss/pkg/controller/clusterchannelprovisioner"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/utils"
)

var (
	readTimeout  = 1 * time.Minute
	writeTimeout = 1 * time.Minute

	port               int
	configMapNoticer   string
	configMapNamespace string
	configMapName      string
)

func main() {

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Unable to create logger: %v", err)
	}

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		logger.Fatal("Error starting up.", zap.Error(err))
	}

	// Add custom types to this array to get them into the manager's scheme.
	eventingv1alpha1.AddToScheme(mgr.GetScheme())

	stopCh := signals.SetupSignalHandler()
	var g errgroup.Group

	logger.Info("Dispatcher starting...")
	natssUrl := fmt.Sprintf(clusterchannelprovisioner.NatssUrlTmpl, utils.GetClusterDomainName())
	dispatcher, err := dispatcher.NewDispatcher(natssUrl, logger)
	if err != nil {
		logger.Fatal("Unable to create NATSS dispatcher.", zap.Error(err))
	}
	// Starting Connect to establish connection with NATS
	go dispatcher.Connect()

	g.Go(func() error {
		return dispatcher.Start(stopCh)
	})

	_, err = channel.ProvideController(dispatcher, mgr, logger)
	if err != nil {
		logger.Fatal("Unable to create Channel controller", zap.Error(err))
	}

	logger.Info("Dispatcher controller starting...")

	err = mgr.Start(stopCh)
	if err != nil {
		logger.Fatal("Manager.Start() returned an error", zap.Error(err))
	}
}
