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

	"k8s.io/apimachinery/pkg/runtime/schema"
	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/defaultchannel"
	"knative.dev/eventing/pkg/logconfig"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	tracingconfig "knative.dev/pkg/tracing/config"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/configmaps"
	"knative.dev/pkg/webhook/resourcesemantics"
)

func NewResourceAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	logger := logging.FromContext(ctx)

	// Decorate contexts with the current state of the config.
	// store := defaultconfig.NewStore(logging.FromContext(ctx).Named("config-store"))
	// store.WatchConfigs(cmw)
	ctxFunc := func(ctx context.Context) context.Context {
		// return v1.WithUpgradeViaDefaulting(store.ToContext(ctx))
		return ctx
	}

	// Watch the default-ch-webhook ConfigMap and dynamically update the default
	// Channel CRD.
	// TODO(#2128): This should be persisted to context in the context function
	// above and fetched off of context by the api code.  See knative/serving's logic
	// around config-defaults for an example of this.
	chDefaulter := defaultchannel.New(logger.Desugar())
	eventingduckv1alpha1.ChannelDefaulterSingleton = chDefaulter
	cmw.Watch(defaultchannel.ConfigMapName, chDefaulter.UpdateConfigMap)

	return resourcesemantics.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"webhook.eventing.knative.dev",

		// The path on which to serve the webhook.
		"/",

		// The resources to validate and default.
		map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
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
		},

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		ctxFunc,

		// Whether to disallow unknown fields.
		true,
	)
}

func NewConfigValidationController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return configmaps.NewAdmissionController(ctx,

		// Name of the configmap webhook.
		"config.webhook.eventing.knative.dev",

		// The path on which to serve the webhook.
		"/config-validation",

		// The configmaps to validate.
		configmap.Constructors{
			tracingconfig.ConfigName: tracingconfig.NewTracingConfigFromConfigMap,
			// metrics.ConfigMapName():   metricsconfig.NewObservabilityConfigFromConfigMap,
			logging.ConfigMapName(): logging.NewConfigFromConfigMap,
		},
	)
}

func main() {
	// Set up a signal context with our webhook options
	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		ServiceName: logconfig.WebhookName(),
		Port:        8443,
		SecretName:  "eventing-webhook-certs",
	})

	sharedmain.MainWithContext(ctx, logconfig.WebhookName(),
		certificates.NewController,
		NewResourceAdmissionController,
		NewConfigValidationController,
	)
}
