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
	"flag"
	"log"
	"os"

	"k8s.io/client-go/tools/clientcmd"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/eventing/pkg/duck"
	"github.com/knative/eventing/pkg/logconfig"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/metrics"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/eventing/pkg/reconciler/broker"
	"github.com/knative/eventing/pkg/reconciler/channel"
	"github.com/knative/eventing/pkg/reconciler/eventtype"
	"github.com/knative/eventing/pkg/reconciler/namespace"
	"github.com/knative/eventing/pkg/reconciler/subscription"
	"github.com/knative/eventing/pkg/reconciler/trigger"
	"github.com/knative/pkg/configmap"
	kncontroller "github.com/knative/pkg/controller"
	pkgmetrics "github.com/knative/pkg/metrics"
	"github.com/knative/pkg/signals"
	"go.uber.org/zap"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
)

const (
	component = "controller"
)

var (
	hardcodedLoggingConfig = flag.Bool("hardCodedLoggingConfig", false, "If true, use the hard coded logging config. It is intended to be used only when debugging outside a Kubernetes cluster.")
	masterURL              = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	kubeconfig             = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
)

func main() {
	flag.Parse()

	logger, atomicLevel := setupLogger()
	defer flush(logger)

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
	if err != nil {
		logger.Fatalw("Error building kubeconfig", zap.Error(err))
	}

	logger.Info("Starting the controller")

	const numControllers = 6
	cfg.QPS = numControllers * rest.DefaultQPS
	cfg.Burst = numControllers * rest.DefaultBurst
	opt := reconciler.NewOptionsOrDie(cfg, logger, stopCh)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(opt.KubeClientSet, opt.ResyncPeriod)
	eventingInformerFactory := informers.NewSharedInformerFactory(opt.EventingClientSet, opt.ResyncPeriod)
	apiExtensionsInformerFactory := apiextensionsinformers.NewSharedInformerFactory(opt.ApiExtensionsClientSet, opt.ResyncPeriod)

	// Eventing
	triggerInformer := eventingInformerFactory.Eventing().V1alpha1().Triggers()
	channelInformer := eventingInformerFactory.Eventing().V1alpha1().Channels()
	subscriptionInformer := eventingInformerFactory.Eventing().V1alpha1().Subscriptions()
	brokerInformer := eventingInformerFactory.Eventing().V1alpha1().Brokers()
	eventTypeInformer := eventingInformerFactory.Eventing().V1alpha1().EventTypes()

	// Kube
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	namespaceInformer := kubeInformerFactory.Core().V1().Namespaces()
	configMapInformer := kubeInformerFactory.Core().V1().ConfigMaps()
	deploymentInformer := kubeInformerFactory.Apps().V1().Deployments()
	serviceAccountInformer := kubeInformerFactory.Core().V1().ServiceAccounts()
	roleBindingInformer := kubeInformerFactory.Rbac().V1().RoleBindings()

	// Api Extensions
	customResourceDefinitionInformer := apiExtensionsInformerFactory.Apiextensions().V1beta1().CustomResourceDefinitions()

	// Duck
	addressableInformer := duck.NewAddressableInformer(opt)

	// Build all of our controllers, with the clients constructed above.
	// Add new controllers to this array.
	// You also need to modify numControllers above to match this.
	controllers := [...]*kncontroller.Impl{
		subscription.NewController(
			opt,
			subscriptionInformer,
			addressableInformer,
			customResourceDefinitionInformer,
		),
		namespace.NewController(
			opt,
			namespaceInformer,
			serviceAccountInformer,
			roleBindingInformer,
			brokerInformer,
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
			serviceInformer,
			addressableInformer,
		),
		broker.NewController(
			opt,
			brokerInformer,
			subscriptionInformer,
			channelInformer,
			serviceInformer,
			deploymentInformer,
			broker.ReconcilerArgs{
				IngressImage:              getRequiredEnv("BROKER_INGRESS_IMAGE"),
				IngressServiceAccountName: getRequiredEnv("BROKER_INGRESS_SERVICE_ACCOUNT"),
				FilterImage:               getRequiredEnv("BROKER_FILTER_IMAGE"),
				FilterServiceAccountName:  getRequiredEnv("BROKER_FILTER_SERVICE_ACCOUNT"),
			},
		),
		eventtype.NewController(
			opt,
			eventTypeInformer,
			brokerInformer,
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
	// Watch the observability config map and dynamically update metrics exporter.
	opt.ConfigMapWatcher.Watch(metrics.ObservabilityConfigName, metrics.UpdateExporterFromConfigMap(component, logger))
	if err := opt.ConfigMapWatcher.Start(stopCh); err != nil {
		logger.Fatalw("failed to start configuration manager", zap.Error(err))
	}

	// Start all of the informers and wait for them to sync.
	logger.Info("Starting informers.")
	if err := kncontroller.StartInformers(
		stopCh,
		// Eventing
		brokerInformer.Informer(),
		channelInformer.Informer(),
		subscriptionInformer.Informer(),
		triggerInformer.Informer(),
		eventTypeInformer.Informer(),
		// Kube
		configMapInformer.Informer(),
		serviceInformer.Informer(),
		namespaceInformer.Informer(),
		deploymentInformer.Informer(),
		serviceAccountInformer.Informer(),
		roleBindingInformer.Informer(),
		// Api Extensions
		customResourceDefinitionInformer.Informer(),
	); err != nil {
		logger.Fatalf("Failed to start informers: %v", err)
	}

	// Start all of the controllers.
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

func getRequiredEnv(envKey string) string {
	val, defined := os.LookupEnv(envKey)
	if !defined {
		log.Fatalf("required environment variable not defined '%s'", envKey)
	}
	return val
}

func flush(logger *zap.SugaredLogger) {
	logger.Sync()
	pkgmetrics.FlushExporter()
}
