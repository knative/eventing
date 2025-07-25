/*
Copyright 2020 The Knative Authors

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

package broker

import (
	"context"

	"knative.dev/eventing/pkg/auth"

	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/clients/dynamicclient"
	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"
	namespacedinformerfactory "knative.dev/pkg/injection/clients/namespacedkube/informers/factory"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/system"

	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/feature"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1/channelable"
	brokerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/broker"
	eventpolicyinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventpolicy"
	subscriptioninformer "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/subscription"
	brokerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/broker"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler/names"
)

const (
	BrokerConditionReady                             = apis.ConditionReady
	BrokerConditionIngress        apis.ConditionType = "IngressReady"
	BrokerConditionTriggerChannel apis.ConditionType = "TriggerChannelReady"
	BrokerConditionFilter         apis.ConditionType = "FilterReady"
	BrokerConditionAddressable    apis.ConditionType = "Addressable"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)
	brokerInformer := brokerinformer.Get(ctx)
	subscriptionInformer := subscriptioninformer.Get(ctx)

	endpointsInformer := namespacedinformerfactory.Get(ctx).Core().V1().Endpoints()
	if err := controller.StartInformers(ctx.Done(), endpointsInformer.Informer()); err != nil {
		logger.Fatalw("Failed to start namespaced endpoints informer", zap.Error(err))
	}

	configmapInformer := configmapinformer.Get(ctx)
	secretInformer := secretinformer.Get(ctx)
	eventPolicyInformer := eventpolicyinformer.Get(ctx)

	var globalResync func(obj interface{})

	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"), func(name string, value interface{}) {
		if globalResync != nil {
			globalResync(nil)
		}
	})
	featureStore.WatchConfigs(cmw)

	eventingv1.RegisterAlternateBrokerConditionSet(apis.NewLivingConditionSet(
		BrokerConditionIngress,
		BrokerConditionTriggerChannel,
		BrokerConditionFilter,
		BrokerConditionAddressable,
		eventingv1.BrokerConditionEventPoliciesReady,
	))

	brokerFilter := pkgreconciler.AnnotationFilterFunc(brokerreconciler.ClassAnnotationKey, eventing.MTChannelBrokerClassValue, false /*allowUnset*/)

	r := &Reconciler{
		eventingClientSet:  eventingclient.Get(ctx),
		dynamicClientSet:   dynamicclient.Get(ctx),
		endpointsLister:    endpointsInformer.Lister(),
		subscriptionLister: subscriptionInformer.Lister(),
		brokerClass:        eventing.MTChannelBrokerClassValue,
		configmapLister:    configmapInformer.Lister(),
		secretLister:       secretInformer.Lister(),
		eventPolicyLister:  eventPolicyInformer.Lister(),
	}
	impl := brokerreconciler.NewImpl(ctx, r, eventing.MTChannelBrokerClassValue, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			ConfigStore:       featureStore,
			PromoteFilterFunc: brokerFilter,
		}
	})

	r.channelableTracker = duck.NewListableTrackerFromTracker(ctx, channelable.Get, impl.Tracker)
	r.uriResolver = resolver.NewURIResolverFromTracker(ctx, impl.Tracker)

	brokerInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: brokerFilter,
		Handler:    controller.HandleAll(impl.Enqueue),
	})

	// When the endpoints in our multi-tenant filter/ingress change, do a global resync.
	// During installation, we might reconcile Brokers before our shared filter/ingress is
	// ready, so when these endpoints change perform a global resync.
	globalResync = func(any) {
		// Since changes in the Filter/Ingress Service endpoints affect all the Broker objects,
		// do a global resync.
		logger.Info("Doing a global resync due to endpoint changes in shared broker component")
		impl.FilteredGlobalResync(brokerFilter, brokerInformer.Informer())
	}
	// Resync for the filter.
	endpointsInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: pkgreconciler.ChainFilterFuncs(
			pkgreconciler.NamespaceFilterFunc(system.Namespace()),
			pkgreconciler.NameFilterFunc(names.BrokerFilterName)),
		Handler: controller.HandleAll(globalResync),
	})
	// Resync for the ingress.
	endpointsInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: pkgreconciler.ChainFilterFuncs(
			pkgreconciler.NamespaceFilterFunc(system.Namespace()),
			pkgreconciler.NameFilterFunc(names.BrokerIngressName)),
		Handler: controller.HandleAll(globalResync),
	})
	secretInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithName(ingressServerTLSSecretName),
		Handler:    controller.HandleAll(globalResync),
	})

	brokerGK := eventingv1.SchemeGroupVersion.WithKind("Broker").GroupKind()

	// Enqueue the Broker, if we have an EventPolicy which was referencing
	// or got updated and now is referencing the Broker
	eventPolicyInformer.Informer().AddEventHandler(auth.EventPolicyEventHandler(brokerInformer.Informer().GetIndexer(), brokerGK, impl.EnqueueKey))

	return impl
}
