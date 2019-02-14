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

	"github.com/knative/eventing/pkg/reconciler/v1alpha1/subscription"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/logconfig"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/logging"
	"github.com/knative/pkg/logging/logkey"
	"github.com/knative/pkg/signals"
	"github.com/knative/pkg/system"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	controllerruntime "sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

const (
	metricsScrapeAddr = ":9090"
	metricsScrapePath = "/metrics"
)

var (
	hardcodedLoggingConfig bool
)

// SchemeFunc adds types to a Scheme.
type SchemeFunc func(*runtime.Scheme) error

// ProvideFunc adds a controller to a Manager.
type ProvideFunc func(manager.Manager) (controller.Controller, error)

func main() {
	flag.Parse()
	logf.SetLogger(logf.ZapLogger(false))

	// Read the logging config and setup a logger.
	cm := getLoggingConfigOrDie()

	config, err := logging.NewConfigFromMap(cm, logconfig.Controller)
	if err != nil {
		log.Fatalf("Error parsing logging configuration: %v", err)
	}
	logger, atomicLevel := logging.NewLoggerFromConfig(config, logconfig.Controller)
	defer logger.Sync()
	logger = logger.With(zap.String(logkey.ControllerType, logconfig.Controller))

	logger.Info("Starting the controller")

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

	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher := configmap.NewInformedWatcher(kubeClient, system.Namespace())
	configMapWatcher.Watch(logconfig.ConfigName, logging.UpdateLevelFromConfigMap(logger, atomicLevel, logconfig.Controller, logconfig.Controller))
	if err = configMapWatcher.Start(stopCh); err != nil {
		logger.Fatalf("Failed to start controller config map watcher: %v", err)
	}

	// Setup a Manager
	mgr, err := manager.New(cfg, manager.Options{})
	if err != nil {
		logger.Fatalf("Failed to create manager: %v", err)
	}

	// Add custom types to this array to get them into the manager's scheme.
	schemeFuncs := []SchemeFunc{
		istiov1alpha3.AddToScheme,
		eventingv1alpha1.AddToScheme,
	}
	for _, schemeFunc := range schemeFuncs {
		if err := schemeFunc(mgr.GetScheme()); err != nil {
			logger.Fatalf("Error adding type to manager's scheme: %v", err)
		}
	}

	// Add each controller's ProvideController func to this list to have the
	// manager run it.
	providers := []ProvideFunc{
		subscription.ProvideController,
	}
	for _, provider := range providers {
		if _, err := provider(mgr); err != nil {
			logger.Fatalf("Error adding controller to manager: %v", err)
		}
	}

	// Start the Manager
	go func() {
		if err := mgr.Start(stopCh); err != nil {
			logger.Fatalf("Error starting manager: %v", err)
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
