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
	"k8s.io/client-go/kubernetes/scheme"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/filtered"

	eventingv1beta3 "knative.dev/eventing/pkg/apis/eventing/v1beta3"
	"knative.dev/eventing/pkg/apis/feature"
	sinksv1alpha1 "knative.dev/eventing/pkg/apis/sinks/v1alpha1"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/eventingtls"

	filteredFactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/leaderelection"
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
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	eventingv1beta2 "knative.dev/eventing/pkg/apis/eventing/v1beta2"
	flowsv1 "knative.dev/eventing/pkg/apis/flows/v1"
	channeldefaultconfig "knative.dev/eventing/pkg/apis/messaging/config"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/apis/sources"
	pingdefaultconfig "knative.dev/eventing/pkg/apis/sources/config"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	sourcesv1beta2 "knative.dev/eventing/pkg/apis/sources/v1beta2"
	"knative.dev/eventing/pkg/apis/sugar"
	"knative.dev/eventing/pkg/reconciler/sinkbinding"

	versionedscheme "knative.dev/eventing/pkg/client/clientset/versioned/scheme"
)

func init() {
	versionedscheme.AddToScheme(scheme.Scheme)
}

var ourTypes = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	// For group eventing.knative.dev.
	// v1beta1
	eventingv1beta1.SchemeGroupVersion.WithKind("EventType"): &eventingv1beta1.EventType{},
	// v1beta2
	eventingv1beta2.SchemeGroupVersion.WithKind("EventType"): &eventingv1beta2.EventType{},
	// v1
	eventingv1.SchemeGroupVersion.WithKind("Broker"):  &eventingv1.Broker{},
	eventingv1.SchemeGroupVersion.WithKind("Trigger"): &eventingv1.Trigger{},

	// For group messaging.knative.dev.
	// v1
	messagingv1.SchemeGroupVersion.WithKind("Channel"):      &messagingv1.Channel{},
	messagingv1.SchemeGroupVersion.WithKind("Subscription"): &messagingv1.Subscription{},

	// For group sources.knative.dev.
	// v1beta2
	sourcesv1beta2.SchemeGroupVersion.WithKind("PingSource"): &sourcesv1beta2.PingSource{},
	// v1
	sourcesv1.SchemeGroupVersion.WithKind("ApiServerSource"): &sourcesv1.ApiServerSource{},
	sourcesv1.SchemeGroupVersion.WithKind("PingSource"):      &sourcesv1.PingSource{},
	sourcesv1.SchemeGroupVersion.WithKind("SinkBinding"):     &sourcesv1.SinkBinding{},
	sourcesv1.SchemeGroupVersion.WithKind("ContainerSource"): &sourcesv1.ContainerSource{},

	// For group sinks.knative.dev.
	// v1alpha1
	sinksv1alpha1.SchemeGroupVersion.WithKind("JobSink"): &sinksv1alpha1.JobSink{},

	// For group flows.knative.dev
	// v1
	flowsv1.SchemeGroupVersion.WithKind("Parallel"): &flowsv1.Parallel{},
	flowsv1.SchemeGroupVersion.WithKind("Sequence"): &flowsv1.Sequence{},
}

var callbacks = map[schema.GroupVersionKind]validation.Callback{}

func NewDefaultingAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	// Decorate contexts with the current state of the config.
	store := defaultconfig.NewStore(logging.FromContext(ctx).Named("config-store"))
	store.WatchConfigs(cmw)

	channelStore := channeldefaultconfig.NewStore(logging.FromContext(ctx).Named("channel-config-store"))
	channelStore.WatchConfigs(cmw)

	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"))
	featureStore.WatchConfigs(cmw)

	// Decorate contexts with the current state of the config.
	ctxFunc := func(ctx context.Context) context.Context {
		return featureStore.ToContext(channelStore.ToContext(store.ToContext(ctx)))
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

	pingstore := pingdefaultconfig.NewStore(logging.FromContext(ctx).Named("ping-config-store"))
	pingstore.WatchConfigs(cmw)

	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"))
	featureStore.WatchConfigs(cmw)

	// Decorate contexts with the current state of the config.
	ctxFunc := func(ctx context.Context) context.Context {
		return featureStore.ToContext(channelStore.ToContext(pingstore.ToContext(store.ToContext(ctx))))
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

func NewConfigValidationController(ctx context.Context, _ configmap.Watcher) *controller.Impl {
	return configmaps.NewAdmissionController(ctx,

		// Name of the configmap webhook.
		"config.webhook.eventing.knative.dev",

		// The path on which to serve the webhook.
		"/config-validation",

		// The configmaps to validate.
		configmap.Constructors{
			tracingconfig.ConfigName: tracingconfig.NewTracingConfigFromConfigMap,
			// metrics.ConfigMapName():   metricsconfig.NewObservabilityConfigFromConfigMap,
			logging.ConfigMapName():        logging.NewConfigFromConfigMap,
			leaderelection.ConfigMapName(): leaderelection.NewConfigFromConfigMap,
			sugar.ConfigName:               sugar.NewConfigFromConfigMap,
		},
	)
}

func NewSinkBindingWebhook(opts ...psbinding.ReconcilerOption) injection.ControllerConstructor {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		trustBundleConfigMapLister := configmapinformer.Get(ctx, eventingtls.TrustBundleLabelSelector).Lister()
		withContext := sinkbinding.WithContextFactory(ctx, trustBundleConfigMapLister, func(types.NamespacedName) {})

		return psbinding.NewAdmissionController(ctx,

			// Name of the resource webhook.
			"sinkbindings.webhook.sources.knative.dev",

			// The path on which to serve the webhook.
			"/sinkbindings",

			// How to get all the Bindables for configuring the mutating webhook.
			sinkbinding.ListAll,

			// How to setup the context prior to invoking Do/Undo.
			withContext,
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

	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"))
	featureStore.WatchConfigs(cmw)

	// Decorate contexts with the current state of the config.
	ctxFunc := func(ctx context.Context) context.Context {
		return featureStore.ToContext(channelStore.ToContext(store.ToContext(ctx)))
	}

	var (
		sourcesv1beta2_  = sourcesv1beta2.SchemeGroupVersion.Version
		sourcesv1_       = sourcesv1.SchemeGroupVersion.Version
		eventingv1beta1_ = eventingv1beta1.SchemeGroupVersion.Version
		eventingv1beta2_ = eventingv1beta2.SchemeGroupVersion.Version
		eventingv1beta3_ = eventingv1beta3.SchemeGroupVersion.Version
	)

	return conversion.NewConversionController(ctx,
		// The path on which to serve the webhook
		"/resource-conversion",

		// Specify the types of custom resource definitions that should be converted
		map[schema.GroupKind]conversion.GroupKindConversion{
			// Sources
			sourcesv1.Kind("PingSource"): {
				DefinitionName: sources.PingSourceResource.String(),
				HubVersion:     sourcesv1beta2_,
				Zygotes: map[string]conversion.ConvertibleObject{
					sourcesv1beta2_: &sourcesv1beta2.PingSource{},
					sourcesv1_:      &sourcesv1.PingSource{},
				},
			},
			// Eventing
			eventingv1beta2.Kind("EventType"): {
				DefinitionName: eventing.EventTypesResource.String(),
				HubVersion:     eventingv1beta1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					eventingv1beta1_: &eventingv1beta1.EventType{},
					eventingv1beta2_: &eventingv1beta2.EventType{},
					eventingv1beta3_: &eventingv1beta3.EventType{},
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
		ServiceName: webhook.NameFromEnv(),
		Port:        webhook.PortFromEnv(8443),
		// SecretName must match the name of the Secret created in the configuration.
		SecretName: "eventing-webhook-certs",
	})

	ctx = filteredFactory.WithSelectors(ctx,
		auth.OIDCLabelSelector,
		eventingtls.TrustBundleLabelSelector,
	)

	sharedmain.WebhookMainWithContext(ctx, webhook.NameFromEnv(),
		certificates.NewController,
		NewConfigValidationController,
		NewValidationAdmissionController,
		NewDefaultingAdmissionController,
		NewConversionController,

		// For each binding we have a controller and a binding webhook.
		sinkbinding.NewController, NewSinkBindingWebhook(sbSelector),
	)
}
