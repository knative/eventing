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
	"os"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"knative.dev/eventing/pkg/logconfig"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	kle "knative.dev/pkg/leaderelection"
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

	defaultconfig "knative.dev/eventing/pkg/apis/config"
	configsv1alpha1 "knative.dev/eventing/pkg/apis/configs/v1alpha1"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/eventing/pkg/apis/flows"
	flowsv1alpha1 "knative.dev/eventing/pkg/apis/flows/v1alpha1"
	flowsv1beta1 "knative.dev/eventing/pkg/apis/flows/v1beta1"
	"knative.dev/eventing/pkg/apis/messaging"
	channeldefaultconfig "knative.dev/eventing/pkg/apis/messaging/config"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/eventing/pkg/apis/sources"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"
	"knative.dev/eventing/pkg/leaderelection"
	"knative.dev/eventing/pkg/reconciler/sinkbinding"
)

var ourTypes = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	// For group eventing.knative.dev.
	// v1alpha1
	eventingv1alpha1.SchemeGroupVersion.WithKind("Broker"):    &eventingv1alpha1.Broker{},
	eventingv1alpha1.SchemeGroupVersion.WithKind("Trigger"):   &eventingv1alpha1.Trigger{},
	eventingv1alpha1.SchemeGroupVersion.WithKind("EventType"): &eventingv1alpha1.EventType{},
	// v1beta1
	eventingv1beta1.SchemeGroupVersion.WithKind("Broker"):    &eventingv1beta1.Broker{},
	eventingv1beta1.SchemeGroupVersion.WithKind("Trigger"):   &eventingv1beta1.Trigger{},
	eventingv1beta1.SchemeGroupVersion.WithKind("EventType"): &eventingv1beta1.EventType{},

	// For group messaging.knative.dev.
	// v1alpha1
	messagingv1alpha1.SchemeGroupVersion.WithKind("InMemoryChannel"): &messagingv1alpha1.InMemoryChannel{},
	messagingv1alpha1.SchemeGroupVersion.WithKind("Channel"):         &messagingv1alpha1.Channel{},
	messagingv1alpha1.SchemeGroupVersion.WithKind("Subscription"):    &messagingv1alpha1.Subscription{},
	// v1beta1
	messagingv1beta1.SchemeGroupVersion.WithKind("InMemoryChannel"): &messagingv1beta1.InMemoryChannel{},
	messagingv1beta1.SchemeGroupVersion.WithKind("Channel"):         &messagingv1beta1.Channel{},
	messagingv1beta1.SchemeGroupVersion.WithKind("Subscription"):    &messagingv1beta1.Subscription{},

	// For group sources.knative.dev.
	// v1alpha1
	sourcesv1alpha1.SchemeGroupVersion.WithKind("ApiServerSource"): &sourcesv1alpha1.ApiServerSource{},
	sourcesv1alpha1.SchemeGroupVersion.WithKind("PingSource"):      &sourcesv1alpha1.PingSource{},
	sourcesv1alpha1.SchemeGroupVersion.WithKind("SinkBinding"):     &sourcesv1alpha1.SinkBinding{},
	// v1alpha2
	sourcesv1alpha2.SchemeGroupVersion.WithKind("ApiServerSource"): &sourcesv1alpha2.ApiServerSource{},
	sourcesv1alpha2.SchemeGroupVersion.WithKind("PingSource"):      &sourcesv1alpha2.PingSource{},
	sourcesv1alpha2.SchemeGroupVersion.WithKind("SinkBinding"):     &sourcesv1alpha2.SinkBinding{},
	sourcesv1alpha2.SchemeGroupVersion.WithKind("ContainerSource"): &sourcesv1alpha2.ContainerSource{},

	// For group flows.knative.dev
	// v1alpha1
	flowsv1alpha1.SchemeGroupVersion.WithKind("Parallel"): &flowsv1alpha1.Parallel{},
	flowsv1alpha1.SchemeGroupVersion.WithKind("Sequence"): &flowsv1alpha1.Sequence{},
	// v1beta1
	flowsv1beta1.SchemeGroupVersion.WithKind("Parallel"): &flowsv1beta1.Parallel{},
	flowsv1beta1.SchemeGroupVersion.WithKind("Sequence"): &flowsv1beta1.Sequence{},

	// For group configs.knative.dev
	configsv1alpha1.SchemeGroupVersion.WithKind("ConfigMapPropagation"): &configsv1alpha1.ConfigMapPropagation{},
}

var callbacks = map[schema.GroupVersionKind]validation.Callback{}

func NewDefaultingAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	// Decorate contexts with the current state of the config.
	store := defaultconfig.NewStore(logging.FromContext(ctx).Named("config-store"))
	store.WatchConfigs(cmw)

	channelStore := channeldefaultconfig.NewStore(logging.FromContext(ctx).Named("channel-config-store"))
	channelStore.WatchConfigs(cmw)

	// Decorate contexts with the current state of the config.
	ctxFunc := func(ctx context.Context) context.Context {
		return channelStore.ToContext(store.ToContext(ctx))
	}

	return defaulting.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"webhook.eventing.knative.dev",

		// The path on which to serve the webhook.
		"/defaulting",

		// The resources to default.
		ourTypes,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		ctxFunc,

		// Whether to disallow unknown fields.
		true,
	)
}

func NewValidationAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	// Decorate contexts with the current state of the config.
	store := defaultconfig.NewStore(logging.FromContext(ctx).Named("config-store"))
	store.WatchConfigs(cmw)

	channelStore := channeldefaultconfig.NewStore(logging.FromContext(ctx).Named("channel-config-store"))
	channelStore.WatchConfigs(cmw)

	// Decorate contexts with the current state of the config.
	ctxFunc := func(ctx context.Context) context.Context {
		return channelStore.ToContext(store.ToContext(ctx))
	}

	return validation.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"validation.webhook.eventing.knative.dev",

		// The path on which to serve the webhook.
		"/resource-validation",

		// The resources to validate.
		ourTypes,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		ctxFunc,

		// Whether to disallow unknown fields.
		true,

		// Extra validating callbacks to be applied to resources.
		callbacks,
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
			kle.ConfigMapName():     leaderelection.ValidateConfig,
		},
	)
}

func NewSinkBindingWebhook(opts ...psbinding.ReconcilerOption) injection.ControllerConstructor {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
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
			opts...,
		)
	}
}

func NewConversionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	// Decorate contexts with the current state of the config.
	store := defaultconfig.NewStore(logging.FromContext(ctx).Named("config-store"))
	store.WatchConfigs(cmw)

	channelStore := channeldefaultconfig.NewStore(logging.FromContext(ctx).Named("channel-config-store"))
	channelStore.WatchConfigs(cmw)

	// Decorate contexts with the current state of the config.
	ctxFunc := func(ctx context.Context) context.Context {
		return channelStore.ToContext(store.ToContext(ctx))
	}

	var (
		eventingv1alpha1_  = eventingv1alpha1.SchemeGroupVersion.Version
		eventingv1beta1_   = eventingv1beta1.SchemeGroupVersion.Version
		messagingv1alpha1_ = messagingv1alpha1.SchemeGroupVersion.Version
		messagingv1beta1_  = messagingv1beta1.SchemeGroupVersion.Version
		flowsv1alpha1_     = flowsv1alpha1.SchemeGroupVersion.Version
		flowsv1beta1_      = flowsv1beta1.SchemeGroupVersion.Version
		sourcesv1alpha1_   = sourcesv1alpha1.SchemeGroupVersion.Version
		sourcesv1alpha2_   = sourcesv1alpha2.SchemeGroupVersion.Version
	)

	return conversion.NewConversionController(ctx,
		// The path on which to serve the webhook
		"/resource-conversion",

		// Specify the types of custom resource definitions that should be converted
		map[schema.GroupKind]conversion.GroupKindConversion{
			// eventing
			eventingv1beta1.Kind("Trigger"): {
				DefinitionName: eventing.TriggersResource.String(),
				HubVersion:     eventingv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					eventingv1alpha1_: &eventingv1alpha1.Trigger{},
					eventingv1beta1_:  &eventingv1beta1.Trigger{},
				},
			},
			eventingv1beta1.Kind("Broker"): {
				DefinitionName: eventing.BrokersResource.String(),
				HubVersion:     eventingv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					eventingv1alpha1_: &eventingv1alpha1.Broker{},
					eventingv1beta1_:  &eventingv1beta1.Broker{},
				},
			},
			eventingv1beta1.Kind("EventType"): {
				DefinitionName: eventing.EventTypesResource.String(),
				HubVersion:     eventingv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					eventingv1alpha1_: &eventingv1alpha1.EventType{},
					eventingv1beta1_:  &eventingv1beta1.EventType{},
				},
			},
			// messaging
			messagingv1beta1.Kind("Channel"): {
				DefinitionName: messaging.ChannelsResource.String(),
				HubVersion:     messagingv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					messagingv1alpha1_: &messagingv1alpha1.Channel{},
					messagingv1beta1_:  &messagingv1beta1.Channel{},
				},
			},
			messagingv1beta1.Kind("InMemoryChannel"): {
				DefinitionName: messaging.InMemoryChannelsResource.String(),
				HubVersion:     messagingv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					messagingv1alpha1_: &messagingv1alpha1.InMemoryChannel{},
					messagingv1beta1_:  &messagingv1beta1.InMemoryChannel{},
				},
			},
			// flows
			flowsv1beta1.Kind("Sequence"): {
				DefinitionName: flows.SequenceResource.String(),
				HubVersion:     flowsv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					flowsv1alpha1_: &flowsv1alpha1.Sequence{},
					flowsv1beta1_:  &flowsv1beta1.Sequence{},
				},
			},
			flowsv1beta1.Kind("Parallel"): {
				DefinitionName: flows.ParallelResource.String(),
				HubVersion:     flowsv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					flowsv1alpha1_: &flowsv1alpha1.Parallel{},
					flowsv1beta1_:  &flowsv1beta1.Parallel{},
				},
			},
			// Sources
			sourcesv1alpha2.Kind("ApiServerSource"): {
				DefinitionName: sources.ApiServerSourceResource.String(),
				HubVersion:     sourcesv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					sourcesv1alpha1_: &sourcesv1alpha1.ApiServerSource{},
					sourcesv1alpha2_: &sourcesv1alpha2.ApiServerSource{},
				},
			},
			sourcesv1alpha2.Kind("PingSource"): {
				DefinitionName: sources.PingSourceResource.String(),
				HubVersion:     sourcesv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					sourcesv1alpha1_: &sourcesv1alpha1.PingSource{},
					sourcesv1alpha2_: &sourcesv1alpha2.PingSource{},
				},
			},
			sourcesv1alpha2.Kind("SinkBinding"): {
				DefinitionName: sources.SinkBindingResource.String(),
				HubVersion:     sourcesv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					sourcesv1alpha1_: &sourcesv1alpha1.SinkBinding{},
					sourcesv1alpha2_: &sourcesv1alpha2.SinkBinding{},
				},
			},
		},

		// A function that infuses the context passed to ConvertTo/ConvertFrom/SetDefaults with custom metadata.
		ctxFunc,
	)
}

func main() {
	sbSelector := psbinding.WithSelector(psbinding.ExclusionSelector)
	if os.Getenv("SINK_BINDING_SELECTION_MODE") == "inclusion" {
		sbSelector = psbinding.WithSelector(psbinding.InclusionSelector)
	}
	// Set up a signal context with our webhook options
	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		ServiceName: logconfig.WebhookName(),
		Port:        8443,
		// SecretName must match the name of the Secret created in the configuration.
		SecretName: "eventing-webhook-certs",
	})

	sharedmain.WebhookMainWithContext(ctx, logconfig.WebhookName(),
		certificates.NewController,
		NewConfigValidationController,
		NewValidationAdmissionController,
		NewDefaultingAdmissionController,
		NewConversionController,

		// For each binding we have a controller and a binding webhook.
		sinkbinding.NewController, NewSinkBindingWebhook(sbSelector),
	)
}
