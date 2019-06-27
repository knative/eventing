/*
Copyright 2019 The Knative Authors

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
	"flag"
	"log"

	"github.com/knative/eventing/pkg/tracing"

	"github.com/knative/eventing/contrib/natss/pkg/util"

	clientset "github.com/knative/eventing/contrib/natss/pkg/client/clientset/versioned"
	informers "github.com/knative/eventing/contrib/natss/pkg/client/informers/externalversions"
	"github.com/knative/eventing/contrib/natss/pkg/dispatcher"
	"github.com/knative/eventing/contrib/natss/pkg/reconciler"
	natsschannel "github.com/knative/eventing/contrib/natss/pkg/reconciler/dispatcher"
	"github.com/knative/eventing/pkg/logconfig"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"knative.dev/pkg/configmap"
	kncontroller "knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
)

var (
	hardcodedLoggingConfig = flag.Bool("hardCodedLoggingConfig", false, "If true, use the hard coded logging config. It is intended to be used only when debugging outside a Kubernetes cluster.")
	masterURL              = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	kubeconfig             = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
)

const (
	clientID = "natss-ch-dispatcher"
)

func main() {
	flag.Parse()
	logger, atomicLevel := setupLogger()
	defer logger.Sync()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
	if err != nil {
		logger.Fatalw("Error building kubeconfig", zap.Error(err))
	}

	natssDispatcher, err := dispatcher.NewDispatcher(util.GetDefaultNatssURL(), util.GetDefaultClusterID(), clientID, logger.Desugar())
	if err != nil {
		logger.Fatalw("Unable to create natss dispatcher", zap.Error(err))
	}

	logger = logger.With(zap.String("controller/impl", "pkg"))
	logger.Info("Starting the NATSS dispatcher")

	const numControllers = 1
	cfg.QPS = numControllers * rest.DefaultQPS
	cfg.Burst = numControllers * rest.DefaultBurst
	opt := reconciler.NewOptionsOrDie(cfg, logger, stopCh)
	messagingClientSet := clientset.NewForConfigOrDie(cfg)
	messagingInformerFactory := informers.NewSharedInformerFactory(messagingClientSet, opt.ResyncPeriod)

	// Messaging
	natssChannelInformer := messagingInformerFactory.Messaging().V1alpha1().NatssChannels()

	// Build all of our controllers, with the clients constructed above.
	// Add new controllers to this array.
	// You also need to modify numControllers above to match this.
	controllers := [...]*kncontroller.Impl{
		natsschannel.NewController(
			opt,
			natssDispatcher,
			natssChannelInformer,
		),
	}
	// This line asserts at compile time that the length of controllers is equal to numControllers.
	// It is based on https://go101.org/article/tips.html#assert-at-compile-time, which notes that
	// var _ [N-M]int
	// asserts at compile time that N >= M, which we can use to establish equality of N and M:
	// (N >= M) && (M >= N) => (N == M)
	var _ [numControllers - len(controllers)][len(controllers) - numControllers]int

	// Watch the logging config map and dynamically update logging levels.
	opt.ConfigMapWatcher.Watch(logconfig.ConfigMapName(), logging.UpdateLevelFromConfigMap(logger, atomicLevel, logconfig.Controller))
	// TODO: Watch the observability config map and dynamically update metrics exporter.
	//opt.ConfigMapWatcher.Watch(metrics.ObservabilityConfigName, metrics.UpdateExporterFromConfigMap(component, logger))

	// Setup zipkin tracing.
	if err = tracing.SetupDynamicZipkinPublishing(logger, opt.ConfigMapWatcher, "natss-ch-dispatcher"); err != nil {
		logger.Fatalw("Error setting up Zipkin publishing", zap.Error(err))
	}

	if err := opt.ConfigMapWatcher.Start(stopCh); err != nil {
		logger.Fatalw("failed to start configuration manager", zap.Error(err))
	}

	// Start all of the informers and wait for them to sync.
	logger.Info("Starting informers.")
	if err := kncontroller.StartInformers(
		stopCh,
		// Messaging
		natssChannelInformer.Informer(),
	); err != nil {
		logger.Fatalf("Failed to start informers: %v", err)
	}

	logger.Info("Starting dispatcher.")
	go natssDispatcher.Start(stopCh)

	logger.Info("Starting controllers.")
	kncontroller.StartAll(stopCh, controllers[:]...)
}

func setupLogger() (*zap.SugaredLogger, zap.AtomicLevel) {
	// Set up our logger.
	loggingConfigMap := getLoggingConfigOrDie()
	loggingConfig, err := logging.NewConfigFromMap(loggingConfigMap)
	if err != nil {
		log.Fatalf("Error parsing logging configuration: %v", err)
	}
	return logging.NewLoggerFromConfig(loggingConfig, logconfig.Controller)
}

func getLoggingConfigOrDie() map[string]string {
	if hardcodedLoggingConfig != nil && *hardcodedLoggingConfig {
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
