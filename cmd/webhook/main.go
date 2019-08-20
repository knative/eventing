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
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime/schema"
	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/defaultchannel"
	"knative.dev/eventing/pkg/logconfig"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/clients/kubeclient"
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

	cfg, err := sharedmain.GetConfig(*masterURL, *kubeconfig)
	if err != nil {
		log.Fatal("Failed to get cluster config:", err)
	}

	log.Printf("Registering %d clients", len(injection.Default.GetClients()))
	log.Printf("Registering %d informer factories", len(injection.Default.GetInformerFactories()))
	log.Printf("Registering %d informers", len(injection.Default.GetInformers()))

	ctx, _ = injection.Default.SetupInformers(ctx, cfg)
	kubeClient := kubeclient.Get(ctx)

	config, err := sharedmain.GetLoggingConfig(ctx)
	if err != nil {
		log.Fatal("Error loading/parsing logging configuration:", err)
	}
	logger, atomicLevel := logging.NewLoggerFromConfig(config, logconfig.WebhookName())
	defer logger.Sync()
	logger = logger.With(zap.String(logkey.ControllerType, logconfig.WebhookName()))

	if err := version.CheckMinimumVersion(kubeClient.Discovery()); err != nil {
		logger.Fatalw("Version check failed", err)
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
		log.Fatal("Failed to process env var", zap.Error(err))
	}
	registrationDelay := getRegistrationDelayTime(env.RegistrationDelayTime)

	stats, err := webhook.NewStatsReporter()
	if err != nil {
		logger.Fatalw("failed to initialize the stats reporter", zap.Error(err))
	}

	options := webhook.ControllerOptions{
		ServiceName:       logconfig.WebhookName(),
		DeploymentName:    logconfig.WebhookName(),
		Namespace:         system.Namespace(),
		Port:              8443,
		SecretName:        "eventing-webhook-certs",
		WebhookName:       "webhook.eventing.knative.dev",
		StatsReporter:     stats,
		RegistrationDelay: registrationDelay * time.Second,
	}

	handlers := map[schema.GroupVersionKind]webhook.GenericCRD{
		// For group eventing.knative.dev,
		eventingv1alpha1.SchemeGroupVersion.WithKind("Broker"):       &eventingv1alpha1.Broker{},
		eventingv1alpha1.SchemeGroupVersion.WithKind("Subscription"): &eventingv1alpha1.Subscription{},
		eventingv1alpha1.SchemeGroupVersion.WithKind("Trigger"):      &eventingv1alpha1.Trigger{},
		eventingv1alpha1.SchemeGroupVersion.WithKind("EventType"):    &eventingv1alpha1.EventType{},
		// For group messaging.knative.dev.
		messagingv1alpha1.SchemeGroupVersion.WithKind("InMemoryChannel"): &messagingv1alpha1.InMemoryChannel{},
		messagingv1alpha1.SchemeGroupVersion.WithKind("Sequence"):        &messagingv1alpha1.Sequence{},
		messagingv1alpha1.SchemeGroupVersion.WithKind("Choice"):          &messagingv1alpha1.Choice{},
		messagingv1alpha1.SchemeGroupVersion.WithKind("Channel"):         &messagingv1alpha1.Channel{},
	}

	// Decorate contexts with the current state of the config.
	ctxFunc := func(ctx context.Context) context.Context {
		// TODO: implement upgrades when eventing needs it:
		//  return v1beta1.WithUpgradeViaDefaulting(store.ToContext(ctx))
		return ctx
	}

	controller, err := webhook.NewAdmissionController(kubeClient, options, handlers, logger, ctxFunc, true)

	if err != nil {
		logger.Fatalw("Failed to create admission controller", zap.Error(err))
	}

	if err = controller.Run(ctx.Done()); err != nil {
		logger.Fatalw("Failed to start the admission controller", zap.Error(err))
	}

	logger.Infow("Webhook stopping")
}
