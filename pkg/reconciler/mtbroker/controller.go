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

package mtbroker

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1/channelable"
	brokerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/broker"
	triggerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/trigger"
	subscriptioninformer "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/subscription"
	brokerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/broker"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler/names"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/client/injection/ducks/duck/v1/conditions"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap"
	endpointsinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/endpoints"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/system"
)

// controllerAgentName is the string used by this controller to identify itself
// when creating events.
const controllerAgentName = "mt-broker-controller"

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
	triggerInformer := triggerinformer.Get(ctx)
	subscriptionInformer := subscriptioninformer.Get(ctx)
	endpointsInformer := endpointsinformer.Get(ctx)
	configmapInformer := configmapinformer.Get(ctx)

	eventingv1.RegisterAlternateBrokerConditionSet(apis.NewLivingConditionSet(
		BrokerConditionIngress,
		BrokerConditionTriggerChannel,
		BrokerConditionFilter,
		BrokerConditionAddressable,
	))

	r := &Reconciler{
		eventingClientSet:  eventingclient.Get(ctx),
		dynamicClientSet:   dynamicclient.Get(ctx),
		endpointsLister:    endpointsInformer.Lister(),
		subscriptionLister: subscriptionInformer.Lister(),
		triggerLister:      triggerInformer.Lister(),
		brokerClass:        eventing.MTChannelBrokerClassValue,
		configmapLister:    configmapInformer.Lister(),
	}
	impl := brokerreconciler.NewImpl(ctx, r, eventing.MTChannelBrokerClassValue)

	logger.Info("Setting up event handlers")

	r.kresourceTracker = duck.NewListableTracker(ctx, conditions.Get, impl.EnqueueKey, controller.GetTrackerLease(ctx))
	r.channelableTracker = duck.NewListableTracker(ctx, channelable.Get, impl.EnqueueKey, controller.GetTrackerLease(ctx))
	r.addressableTracker = duck.NewListableTracker(ctx, addressable.Get, impl.EnqueueKey, controller.GetTrackerLease(ctx))
	r.uriResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	brokerFilter := pkgreconciler.AnnotationFilterFunc(brokerreconciler.ClassAnnotationKey, eventing.MTChannelBrokerClassValue, false /*allowUnset*/)
	brokerInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: brokerFilter,
		Handler:    controller.HandleAll(impl.Enqueue),
	})

	// Reconcile Broker (which transitively reconciles the triggers), when Subscriptions
	// that I own are changed.
	subscriptionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(eventingv1.Kind("Broker")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	// Reconcile trigger (by enqueuing the broker specified in the label) when subscriptions
	// of triggers change.
	subscriptionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: pkgreconciler.LabelExistsFilterFunc(eventing.BrokerLabelKey),
		Handler:    controller.HandleAll(impl.EnqueueLabelOfNamespaceScopedResource("" /*any namespace*/, eventing.BrokerLabelKey)),
	})

	triggerInformer.Informer().AddEventHandler(controller.HandleAll(
		func(obj interface{}) {
			if trigger, ok := obj.(*eventingv1.Trigger); ok {
				impl.EnqueueKey(types.NamespacedName{Namespace: trigger.Namespace, Name: trigger.Spec.Broker})
			}
		},
	))

	// When the endpoints in our multi-tenant filter/ingress change, do a global resync.
	// During installation, we might reconcile Brokers before our shared filter/ingress is
	// ready, so when these endpoints change perform a global resync.
	grCb := func(obj interface{}) {
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
		Handler: controller.HandleAll(grCb),
	})
	// Resync for the ingress.
	endpointsInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: pkgreconciler.ChainFilterFuncs(
			pkgreconciler.NamespaceFilterFunc(system.Namespace()),
			pkgreconciler.NameFilterFunc(names.BrokerIngressName)),
		Handler: controller.HandleAll(grCb),
	})

	return impl
}
