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
	"log"

	"github.com/knative/eventing/contrib/natss/pkg/dispatcher/channel"
	"github.com/knative/eventing/contrib/natss/pkg/dispatcher/dispatcher"
	"github.com/knative/eventing/contrib/natss/pkg/util"
	"github.com/knative/pkg/signals"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
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
	if err = eventingv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		logger.Fatal("Unable to add eventingv1alpha1 scheme", zap.Error(err))
	}

	logger.Info("Dispatcher starting...")
	d, err := dispatcher.NewDispatcher(util.GetDefaultNatssURL(), util.GetDefaultClusterID(), logger)
	if err != nil {
		logger.Fatal("Unable to create NATSS dispatcher.", zap.Error(err))
	}

	if err = mgr.Add(d); err != nil {
		logger.Fatal("Unable to add the dispatcher", zap.Error(err))
	}

	_, err = channel.ProvideController(d, mgr, logger)
	if err != nil {
		logger.Fatal("Unable to create Channel controller", zap.Error(err))
	}

	logger.Info("Dispatcher controller starting...")
	stopCh := signals.SetupSignalHandler()
	err = mgr.Start(stopCh)
	if err != nil {
		logger.Fatal("Manager.Start() returned an error", zap.Error(err))
	}
}
