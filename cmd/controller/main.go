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
	"context"
	"flag"
	"log"
	"net/http"
	"time"

	controllerruntime "sigs.k8s.io/controller-runtime/pkg/client/config"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	clientset "github.com/knative/eventing/pkg/client/clientset/versioned"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/eventing/pkg/controller"
	"github.com/knative/eventing/pkg/controller/bus"
	"github.com/knative/eventing/pkg/controller/channel"
	"github.com/knative/eventing/pkg/controller/clusterbus"
	sharedclientset "github.com/knative/pkg/client/clientset/versioned"
	sharedinformers "github.com/knative/pkg/client/informers/externalversions"
	"github.com/knative/pkg/signals"

	"github.com/knative/eventing/pkg/logconfig"
	"github.com/knative/eventing/pkg/system"
	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/logging"
	"github.com/knative/pkg/logging/logkey"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

const (
	threadsPerController = 2
	metricsScrapeAddr    = ":9090"
	metricsScrapePath    = "/metrics"
)

var (
	experimentalControllers string
	hardcodedLoggingConfig  bool
)

func main() {
	flag.Parse()

	// Read the logging config and setup a logger.
	cm := getLoggingConfigOrDie()

	config, err := logging.NewConfigFromMap(cm, logconfig.Controller)
	if err != nil {
		log.Fatalf("Error parsing logging configuration: %v", err)
	}
	logger, atomicLevel := logging.NewLoggerFromConfig(config, logconfig.Controller)
	defer logger.Sync()
	logger = logger.With(zap.String(logkey.ControllerType, logconfig.Controller))

	logger.Info("Starting the Controller")

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()
	cfg, err := controllerruntime.GetConfig()
	if err != nil {
		logger.Fatalf("Error building kubeconfig: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Fatalf("Error building kubernetes clientset: %v", err)
	}

	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		logger.Fatalf("Error building clientset: %v", err)
	}

	sharedClient, err := sharedclientset.NewForConfig(cfg)
	if err != nil {
		logger.Fatalf("Error building shared clientset: %v", err)
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	informerFactory := informers.NewSharedInformerFactory(client, time.Second*30)
	sharedInformerFactory := sharedinformers.NewSharedInformerFactory(sharedClient, time.Second*30)

	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher := configmap.NewInformedWatcher(kubeClient, system.Namespace)
	configMapWatcher.Watch(logconfig.ConfigName, logging.UpdateLevelFromConfigMap(logger, atomicLevel, logconfig.Controller, logconfig.Controller))
	if err = configMapWatcher.Start(stopCh); err != nil {
		logger.Fatalf("failed to start controller config map watcher: %v", err)
	}

	// Add new controllers here, except controllers that use controller-runtime.
	// Those should be added to controller-runtime-main.go.
	ctors := []controller.Constructor{
		bus.NewController,
		clusterbus.NewController,
		channel.NewController,
	}

	// TODO(n3wscott): Send the logger to the controllers.
	// Build all of our controllers, with the clients constructed above.
	controllers := make([]controller.Interface, 0, len(ctors))
	for _, ctor := range ctors {
		controllers = append(controllers,
			ctor(kubeClient, client, sharedClient, cfg, kubeInformerFactory, informerFactory, sharedInformerFactory))
	}

	go kubeInformerFactory.Start(stopCh)
	go informerFactory.Start(stopCh)
	go sharedInformerFactory.Start(stopCh)

	// Start all of the controllers.
	for _, ctrlr := range controllers {
		go func(ctrlr controller.Interface) {
			// We don't expect this to return until stop is called,
			// but if it does, propagate it back.
			if err := ctrlr.Run(threadsPerController, stopCh); err != nil {
				logger.Fatalf("Error running controller: %v", err)
			}
		}(ctrlr)
	}

	// Start the controller-runtime controllers.
	go func() {
		if err := controllerRuntimeStart(logger, experimentalControllers); err != nil {
			logger.Fatalf("Error running controller-runtime controllers: %v", err)
		}
	}()

	// Start the endpoint that Prometheus scraper talks to
	srv := &http.Server{Addr: metricsScrapeAddr}
	http.Handle(metricsScrapePath, promhttp.Handler())
	go func() {
		logger.Info("Starting metrics listener at %s", metricsScrapeAddr)
		if err := srv.ListenAndServe(); err != nil {
			logger.Infof("Httpserver: ListenAndServe() finished with error: %s", err)
		}
	}()

	<-stopCh

	// Close the http server gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

func init() {
	flag.StringVar(&experimentalControllers, "experimentalControllers", "", "List of experimental controllers to include in the Knative Controller.")
	flag.BoolVar(&hardcodedLoggingConfig, "hardCodedLoggingConfig", false, "If true, use the hard coded logging config. It is intended to be used only when debugging outside a Kubernetes cluster.")
}

func getLoggingConfigOrDie() map[string]string {
	if hardcodedLoggingConfig {
		return map[string]string{
			"loglevel.controller": "info",
			"zap-logger-config": `
				{
					"level": "info",
					"development": false,
					"outputPaths": ["stdout"],
					"errorOutputPaths": ["stderr"],
					"encoding": "json",
					"encoderConfig": {
					"timeKey": "ts",
					"levelKey": "level",
					"nameKey": "logger",
					"callerKey": "caller",
					"messageKey": "msg",
					"stacktraceKey": "stacktrace",
					"lineEnding": "",
					"levelEncoder": "",
					"timeEncoder": "iso8601",
					"durationEncoder": "",
					"callerEncoder": ""
				}`,
		}
	} else {
		cm, err := configmap.Load("/etc/config-logging")
		if err != nil {
			log.Fatalf("Error loading logging configuration: %v", err)
		}
		return cm
	}
}
