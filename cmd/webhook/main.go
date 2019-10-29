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
	"strconv"
	"time"

	"github.com/kelseyhightower/envconfig"
	"go.opencensus.io/stats/view"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime/schema"
	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/defaultchannel"
	"knative.dev/eventing/pkg/logconfig"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/logging/logkey"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	"knative.dev/pkg/version"
	"knative.dev/pkg/webhook"
)

type envConfig struct {
	RegistrationDelayTime string `envconfig:"REG_DELAY_TIME" required:"false"`
}

var (
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
)

func getRegistrationDelayTime(rdt string) time.Duration {
	var RegistrationDelay time.Duration

	if rdt != "" {
		rdtime, err := strconv.ParseInt(rdt, 10, 64)
		if err != nil {
			log.Fatalf("Error ParseInt: %v", err)
		}

		RegistrationDelay = time.Duration(rdtime)
	}

	return RegistrationDelay
}

func main() {
	flag.Parse()

	// Set up signals so we handle the first shutdown signal gracefully.
	ctx := signals.NewContext()

	// Report stats on Go memory usage every 30 seconds.
	msp := metrics.NewMemStatsAll()
	msp.Start(ctx, 30*time.Second)
	if err := view.Register(msp.DefaultViews()...); err != nil {
		log.Fatalf("Error exporting go memstats view: %v", err)
	}

	cfg, err := sharedmain.GetConfig(*masterURL, *kubeconfig)
	if err != nil {
		log.Fatal("Failed to get cluster config:", err)
	}

	log.Printf("Registering %d clients", len(injection.Default.GetClients()))
	log.Printf("Registering %d informer factories", len(injection.Default.GetInformerFactories()))
	log.Printf("Registering %d informers", len(injection.Default.GetInformers()))

	ctx, _ = injection.Default.SetupInformers(ctx, cfg)
	kubeClient := kubeclient.Get(ctx)

	loggingConfig, err := sharedmain.GetLoggingConfig(ctx)
	if err != nil {
		log.Fatal("Error loading/parsing logging configuration:", err)
	}
	logger, atomicLevel := logging.NewLoggerFromConfig(loggingConfig, logconfig.WebhookName())
	defer logger.Sync()
	logger = logger.With(zap.String(logkey.ControllerType, logconfig.WebhookName()))

	if err := version.CheckMinimumVersion(kubeClient.Discovery()); err != nil {
		logger.Fatalw("Version check failed", zap.Error(err))
	}

	logger.Infow("Starting the Eventing Webhook")

	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher := configmap.NewInformedWatcher(kubeClient, system.Namespace())
	// Watch the observability config map and dynamically update metrics exporter.
	configMapWatcher.Watch(metrics.ConfigMapName(), metrics.UpdateExporterFromConfigMap(logconfig.WebhookName(), logger))
	// Watch the observability config map and dynamically update request logs.
	configMapWatcher.Watch(logging.ConfigMapName(), logging.UpdateLevelFromConfigMap(logger, atomicLevel, logconfig.WebhookName()))

	// Watch the default-ch-webhook ConfigMap and dynamically update the default
	// Channel CRD.
	chDefaulter := defaultchannel.New(logger.Desugar())
	eventingduckv1alpha1.ChannelDefaulterSingleton = chDefaulter
	configMapWatcher.Watch(defaultchannel.ConfigMapName, chDefaulter.UpdateConfigMap)

	if err = configMapWatcher.Start(ctx.Done()); err != nil {
		logger.Fatalw("Failed to start the ConfigMap watcher", zap.Error(err))
	}

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logger.Fatalw("Failed to process env var", zap.Error(err))
	}
	registrationDelay := getRegistrationDelayTime(env.RegistrationDelayTime)

	stats, err := webhook.NewStatsReporter()
	if err != nil {
		logger.Fatalw("failed to initialize the stats reporter", zap.Error(err))
	}

	options := webhook.ControllerOptions{
		ServiceName:       logconfig.WebhookName(),
		Namespace:         system.Namespace(),
		Port:              8443,
		SecretName:        "eventing-webhook-certs",
		StatsReporter:     stats,
		RegistrationDelay: registrationDelay * time.Second,

		ResourceMutatingWebhookName:     "webhook.eventing.knative.dev",
		ResourceAdmissionControllerPath: "/",
	}

	resourceHandlers := map[schema.GroupVersionKind]webhook.GenericCRD{
		// For group eventing.knative.dev,
		eventingv1alpha1.SchemeGroupVersion.WithKind("Broker"):    &eventingv1alpha1.Broker{},
		eventingv1alpha1.SchemeGroupVersion.WithKind("Trigger"):   &eventingv1alpha1.Trigger{},
		eventingv1alpha1.SchemeGroupVersion.WithKind("EventType"): &eventingv1alpha1.EventType{},
		// For group messaging.knative.dev.
		messagingv1alpha1.SchemeGroupVersion.WithKind("InMemoryChannel"): &messagingv1alpha1.InMemoryChannel{},
		messagingv1alpha1.SchemeGroupVersion.WithKind("Sequence"):        &messagingv1alpha1.Sequence{},
		messagingv1alpha1.SchemeGroupVersion.WithKind("Parallel"):        &messagingv1alpha1.Parallel{},
		messagingv1alpha1.SchemeGroupVersion.WithKind("Channel"):         &messagingv1alpha1.Channel{},
		messagingv1alpha1.SchemeGroupVersion.WithKind("Subscription"):    &messagingv1alpha1.Subscription{},
		// For group sources.eventing.knative.dev
		sourcesv1alpha1.SchemeGroupVersion.WithKind("ApiServerSource"): &sourcesv1alpha1.ApiServerSource{},
		sourcesv1alpha1.SchemeGroupVersion.WithKind("ContainerSource"): &sourcesv1alpha1.ContainerSource{},
		sourcesv1alpha1.SchemeGroupVersion.WithKind("CronJobSource"):   &sourcesv1alpha1.CronJobSource{},
	}

	// Decorate contexts with the current state of the config.
	ctxFunc := func(ctx context.Context) context.Context {
		// TODO: implement upgrades when eventing needs it:
		//  return v1beta1.WithUpgradeViaDefaulting(store.ToContext(ctx))
		return ctx
	}

	resourceAdmissionController := webhook.NewResourceAdmissionController(resourceHandlers, options, true)
	admissionControllers := map[string]webhook.AdmissionController{
		options.ResourceAdmissionControllerPath: resourceAdmissionController,
	}

	controller, err := webhook.New(kubeClient, options, admissionControllers, logger, ctxFunc)

	if err != nil {
		logger.Fatalw("Failed to create admission controller", zap.Error(err))
	}

	if err = controller.Run(ctx.Done()); err != nil {
		logger.Fatalw("Failed to start the admission controller", zap.Error(err))
	}

	logger.Infow("Webhook stopping")
}
