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

	"knative.dev/eventing/pkg/reconciler/sinkbinding"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	configsv1alpha1 "knative.dev/eventing/pkg/apis/configs/v1alpha1"
	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	flowsv1alpha1 "knative.dev/eventing/pkg/apis/flows/v1alpha1"
	legacysourcesv1alpha1 "knative.dev/eventing/pkg/apis/legacysources/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/defaultchannel"
	"knative.dev/eventing/pkg/logconfig"
	"knative.dev/eventing/pkg/reconciler/legacysinkbinding"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	tracingconfig "knative.dev/pkg/tracing/config"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/configmaps"
	"knative.dev/pkg/webhook/psbinding"
	"knative.dev/pkg/webhook/resourcesemantics"
	"knative.dev/pkg/webhook/resourcesemantics/conversion"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"
)

var ourTypes = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	// For group eventing.knative.dev.
	eventingv1alpha1.SchemeGroupVersion.WithKind("Broker"):    &eventingv1alpha1.Broker{},
	eventingv1alpha1.SchemeGroupVersion.WithKind("Trigger"):   &eventingv1alpha1.Trigger{},
	eventingv1alpha1.SchemeGroupVersion.WithKind("EventType"): &eventingv1alpha1.EventType{},

	// For group messaging.knative.dev.
	messagingv1alpha1.SchemeGroupVersion.WithKind("InMemoryChannel"): &messagingv1alpha1.InMemoryChannel{},
	messagingv1alpha1.SchemeGroupVersion.WithKind("Channel"):         &messagingv1alpha1.Channel{},
	messagingv1alpha1.SchemeGroupVersion.WithKind("Subscription"):    &messagingv1alpha1.Subscription{},

	// For group sources.knative.dev.
	sourcesv1alpha1.SchemeGroupVersion.WithKind("ApiServerSource"): &sourcesv1alpha1.ApiServerSource{},
	sourcesv1alpha1.SchemeGroupVersion.WithKind("PingSource"):      &sourcesv1alpha1.PingSource{},
	sourcesv1alpha1.SchemeGroupVersion.WithKind("SinkBinding"):     &sourcesv1alpha1.SinkBinding{},

	// For group sources.eventing.knative.dev.
	// TODO(#2312): Remove this after v0.13.
	legacysourcesv1alpha1.SchemeGroupVersion.WithKind("ApiServerSource"): &legacysourcesv1alpha1.ApiServerSource{},
	legacysourcesv1alpha1.SchemeGroupVersion.WithKind("ContainerSource"): &legacysourcesv1alpha1.ContainerSource{},
	legacysourcesv1alpha1.SchemeGroupVersion.WithKind("SinkBinding"):     &legacysourcesv1alpha1.SinkBinding{},
	legacysourcesv1alpha1.SchemeGroupVersion.WithKind("CronJobSource"):   &legacysourcesv1alpha1.CronJobSource{},

	// For group flows.knative.dev
	flowsv1alpha1.SchemeGroupVersion.WithKind("Parallel"): &flowsv1alpha1.Parallel{},
	flowsv1alpha1.SchemeGroupVersion.WithKind("Sequence"): &flowsv1alpha1.Sequence{},

	// For group configs.knative.dev
	configsv1alpha1.SchemeGroupVersion.WithKind("ConfigMapPropagation"): &configsv1alpha1.ConfigMapPropagation{},
}

func NewDefaultingAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	logger := logging.FromContext(ctx)

	// Decorate contexts with the current state of the config.
	ctxFunc := func(ctx context.Context) context.Context {
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

	return defaulting.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"webhook.eventing.knative.dev",

		// The path on which to serve the webhook.
		"/defaulting",

		// The resources to validate and default.
		ourTypes,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		ctxFunc,

		// Whether to disallow unknown fields.
		true,
	)
}

func NewValidationAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return validation.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"validation.webhook.eventing.knative.dev",

		// The path on which to serve the webhook.
		"/resource-validation",

		// The resources to validate and default.
		ourTypes,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			// return v1.WithUpgradeViaDefaulting(store.ToContext(ctx))
			return ctx
		},

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

func NewSinkBindingWebhook(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	sbresolver := sinkbinding.WithContextFactory(ctx, func(types.NamespacedName) {})

	return psbinding.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"sinkbindings.webhook.sources.knative.dev",

		// The path on which to serve the webhook.
		"/sinkbindings",

		// How to get all the Bindables for configuring the mutating webhook.
		sinkbinding.ListAll,

		// How to setup the context prior to invoking Do/Undo.
		sbresolver,
	)
}

// TODO(#2312): Remove this after v0.13.
func NewLegacySinkBindingWebhook(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	sbresolver := legacysinkbinding.WithContextFactory(ctx, func(types.NamespacedName) {})

	return psbinding.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"legacysinkbindings.webhook.sources.knative.dev",

		// The path on which to serve the webhook.
		"/legacysinkbindings",

		// How to get all the Bindables for configuring the mutating webhook.
		legacysinkbinding.ListAll,

		// How to setup the context prior to invoking Do/Undo.
		sbresolver,
	)
}

func NewConversionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	var (
		v1alpha1_ = v1alpha1.SchemeGroupVersion.Version
		v1beta1_  = v1beta1.SchemeGroupVersion.Version
	)

	return conversion.NewConversionController(ctx,
		// The path on which to serve the webhook
		"/resource-conversion",

		// Specify the types of custom resource definitions that should be converted
		map[schema.GroupKind]conversion.GroupKindConversion{
			v1beta1.Kind("Trigger"): {
				DefinitionName: eventing.TriggersResource.String(),
				HubVersion:     v1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					v1alpha1_: &v1alpha1.Trigger{},
					v1beta1_:  &v1beta1.Trigger{},
				},
			},
			v1beta1.Kind("Broker"): {
				DefinitionName: eventing.BrokersResource.String(),
				HubVersion:     v1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					v1alpha1_: &v1alpha1.Broker{},
					v1beta1_:  &v1beta1.Broker{},
				},
			},
		},

		// A function that infuses the context passed to ConvertUp/ConvertDown/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			return ctx
		},
	)
}

func main() {
	// Set up a signal context with our webhook options
	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		ServiceName: logconfig.WebhookName(),
		Port:        8443,
		// SecretName must match the name of the Secret created in the configuration.
		SecretName: "eventing-webhook-certs",
	})

	sharedmain.MainWithContext(ctx, logconfig.WebhookName(),
		certificates.NewController,
		NewConfigValidationController,
		NewValidationAdmissionController,
		NewDefaultingAdmissionController,
		NewConversionController,

		// For each binding we have a controller and a binding webhook.
		sinkbinding.NewController, NewSinkBindingWebhook,
		// TODO(#2312): Remove this after v0.13.
		legacysinkbinding.NewController, NewLegacySinkBindingWebhook,
	)
}
