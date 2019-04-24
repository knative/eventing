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
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/eventing/pkg/logconfig"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/eventing/pkg/reconciler/channel"
	"github.com/knative/eventing/pkg/reconciler/namespace"
	"github.com/knative/eventing/pkg/reconciler/subscription"
	"github.com/knative/eventing/pkg/reconciler/trigger"
	"github.com/knative/eventing/pkg/reconciler/v1alpha1/broker"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	"github.com/knative/pkg/configmap"
	kncontroller "github.com/knative/pkg/controller"
	"github.com/knative/pkg/logging/logkey"
	"github.com/knative/pkg/signals"
	"github.com/knative/pkg/system"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	controllerruntime "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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
type ProvideFunc func(manager.Manager, *zap.Logger) (controller.Controller, error)

func main() {
	flag.Parse()
	logf.SetLogger(logf.ZapLogger(false))

	logger, atomicLevel := setupLogger()
	defer logger.Sync()
	logger = logger.With(zap.String(logkey.ControllerType, logconfig.Controller))

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := controllerruntime.GetConfig()
	if err != nil {
		logger.Fatalf("Error building kubeconfig: %v", err)
	}

	go startPkgController(stopCh, cfg, logger, atomicLevel)
	go startControllerRuntime(stopCh, cfg, logger, atomicLevel)
	<-stopCh
}

func startPkgController(stopCh <-chan struct{}, cfg *rest.Config, logger *zap.SugaredLogger, atomicLevel zap.AtomicLevel) {
	logger = logger.With(zap.String("controller/impl", "pkg"))
	logger.Info("Starting the controller")

	const numControllers = 4
	cfg.QPS = numControllers * rest.DefaultQPS
	cfg.Burst = numControllers * rest.DefaultBurst
	opt := reconciler.NewOptionsOrDie(cfg, logger, stopCh)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(opt.KubeClientSet, opt.ResyncPeriod)
	eventingInformerFactory := informers.NewSharedInformerFactory(opt.EventingClientSet, opt.ResyncPeriod)

	triggerInformer := eventingInformerFactory.Eventing().V1alpha1().Triggers()
	channelInformer := eventingInformerFactory.Eventing().V1alpha1().Channels()
	subscriptionInformer := eventingInformerFactory.Eventing().V1alpha1().Subscriptions()
	brokerInformer := eventingInformerFactory.Eventing().V1alpha1().Brokers()
	coreServiceInformer := kubeInformerFactory.Core().V1().Services()
	coreNamespaceInformer := kubeInformerFactory.Core().V1().Namespaces()
	configMapInformer := kubeInformerFactory.Core().V1().ConfigMaps()

	// Build all of our controllers, with the clients constructed above.
	// Add new controllers to this array.
	// You also need to modify numControllers above to match this.
	controllers := []*kncontroller.Impl{
		subscription.NewController(
			opt,
			subscriptionInformer,
		),
		namespace.NewController(
			opt,
			coreNamespaceInformer,
		),
		channel.NewController(
			opt,
			channelInformer,
		),
		trigger.NewController(
			opt,
			triggerInformer,
			channelInformer,
			subscriptionInformer,
			brokerInformer,
			coreServiceInformer,
		),
	}
	if len(controllers) != numControllers {
		logger.Fatalf("Number of controllers and QPS settings mismatch: %d != %d", len(controllers), numControllers)
	}

	// Watch the logging config map and dynamically update logging levels.
	opt.ConfigMapWatcher.Watch(logconfig.ConfigMapName(), logging.UpdateLevelFromConfigMap(logger, atomicLevel, logconfig.Controller))
	// TODO: Watch the observability config map and dynamically update metrics exporter.
	//opt.ConfigMapWatcher.Watch(metrics.ObservabilityConfigName, metrics.UpdateExporterFromConfigMap(component, logger))
	if err := opt.ConfigMapWatcher.Start(stopCh); err != nil {
		logger.Fatalw("failed to start configuration manager", zap.Error(err))
	}

	// Start all of the informers and wait for them to sync.
	logger.Info("Starting informers.")
	if err := kncontroller.StartInformers(
		stopCh,
		subscriptionInformer.Informer(),
		configMapInformer.Informer(),
		coreNamespaceInformer.Informer(),
		triggerInformer.Informer(),
		channelInformer.Informer(),
		brokerInformer.Informer(),
		coreServiceInformer.Informer(),
	); err != nil {
		logger.Fatalf("Failed to start informers: %v", err)
	}

	// Start all of the controllers.
	logger.Info("Starting controllers.")
	go kncontroller.StartAll(stopCh, controllers...)
}

// TODO: remove after done integrating all controllers.
func startControllerRuntime(stopCh <-chan struct{}, cfg *rest.Config, logger *zap.SugaredLogger, atomicLevel zap.AtomicLevel) {
	logger = logger.With(zap.String("controller/impl", "cr"))
	logger.Info("Starting the controller")

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Fatalf("Error building kubernetes clientset: %v", err)
	}

	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher := configmap.NewInformedWatcher(kubeClient, system.Namespace())
	configMapWatcher.Watch(logconfig.ConfigMapName(), logging.UpdateLevelFromConfigMap(logger, atomicLevel, logconfig.Controller))
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
		if err = schemeFunc(mgr.GetScheme()); err != nil {
			logger.Fatalf("Error adding type to manager's scheme: %v", err)
		}
	}

	// Add each controller's ProvideController func to this list to have the
	// manager run it.
	providers := []ProvideFunc{
		broker.ProvideController(
			broker.ReconcilerArgs{
				IngressImage:              getRequiredEnv("BROKER_INGRESS_IMAGE"),
				IngressServiceAccountName: getRequiredEnv("BROKER_INGRESS_SERVICE_ACCOUNT"),
				FilterImage:               getRequiredEnv("BROKER_FILTER_IMAGE"),
				FilterServiceAccountName:  getRequiredEnv("BROKER_FILTER_SERVICE_ACCOUNT"),
			}),
	}
	for _, provider := range providers {
		if _, err = provider(mgr, logger.Desugar()); err != nil {
			logger.Fatalf("Error adding controller to manager: %v", err)
		}
	}

	// Start the Manager
	go func() {
		if localErr := mgr.Start(stopCh); localErr != nil {
			logger.Fatalf("Error starting manager: %v", localErr)
		}
	}()

	// Start the endpoint that Prometheus scraper talks to
	srv := &http.Server{Addr: metricsScrapeAddr}
	http.Handle(metricsScrapePath, promhttp.Handler())
	go func() {
		logger.Infof("Starting metrics listener at %s", metricsScrapeAddr)
		if localErr := srv.ListenAndServe(); localErr != nil {
			logger.Infof("HTTPserver: ListenAndServe() finished with error: %s", localErr)
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

func getRequiredEnv(envKey string) string {
	val, defined := os.LookupEnv(envKey)
	if !defined {
		log.Fatalf("required environment variable not defined '%s'", envKey)
	}
	return val
}
