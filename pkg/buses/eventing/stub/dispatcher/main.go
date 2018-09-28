/*
 * Copyright 2018 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"flag"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/buses"
	"github.com/knative/eventing/pkg/buses/eventing/stub/clusterprovisioner"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	"github.com/knative/pkg/signals"
	"go.uber.org/zap"
	"net/http"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func main() {
	ref := buses.NewBusReferenceFromNames(
		os.Getenv("BUS_NAME"),
		os.Getenv("BUS_NAMESPACE"),
	)

	logConfig := buses.NewLoggingConfig()
	logger := buses.NewBusLoggerFromConfig(logConfig)
	defer logger.Sync()
	logger = logger.With(
		// TODO: probably replace, Bus isn't really a thing anymore.
		zap.String("eventing.knative.dev/bus", ref.String()),
		zap.String("eventing.knative.dev/busType", "stub"),
		zap.String("eventing.knative.dev/busComponent", buses.Dispatcher),
	)
	flag.Parse()


	//config.GetConfigOrDie()

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		logger.Fatal("Error starting up.", zap.Error(err))
	}

	// Add custom types to this array to get them into the manager's scheme.
	eventingv1alpha1.AddToScheme(mgr.GetScheme())
	istiov1alpha3.AddToScheme(mgr.GetScheme())

	_, handler, err := clusterprovisioner.ProvideController(mgr, logger.Desugar())

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()
	// Start blocks forever.
	go mgr.Start(stopCh)

	s := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ErrorLog:     zap.NewStdLog(logger.Desugar()),
	}
	logger.Info("Stub dispatcher started...")

	err = s.ListenAndServe()
	logger.Error("Stub dispatcher stopped", zap.Error(err))
}
