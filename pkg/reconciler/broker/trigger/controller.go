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

package mttrigger

import (
	"context"

	"knative.dev/eventing/pkg/auth"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/client/injection/ducks/duck/v1/source"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/clients/dynamicclient"
	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"

	apiseventing "knative.dev/eventing/pkg/apis/eventing"
	eventing "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/feature"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	brokerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/broker"
	triggerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/trigger"
	subscriptioninformer "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/subscription"
	brokerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/broker"
	triggerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/trigger"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1"
	"knative.dev/eventing/pkg/duck"
	kubeclient "knative.dev/pkg/client/injection/kube/client"

	serviceaccountinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/filtered"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)
	triggerInformer := triggerinformer.Get(ctx)
	brokerInformer := brokerinformer.Get(ctx)
	subscriptionInformer := subscriptioninformer.Get(ctx)
	configmapInformer := configmapinformer.Get(ctx)
	secretInformer := secretinformer.Get(ctx)
	oidcServiceaccountInformer := serviceaccountinformer.Get(ctx, auth.OIDCLabelSelector)

	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"))
	featureStore.WatchConfigs(cmw)

	triggerLister := triggerInformer.Lister()
	r := &Reconciler{
		eventingClientSet:    eventingclient.Get(ctx),
		dynamicClientSet:     dynamicclient.Get(ctx),
		kubeclient:           kubeclient.Get(ctx),
		subscriptionLister:   subscriptionInformer.Lister(),
		brokerLister:         brokerInformer.Lister(),
		triggerLister:        triggerLister,
		configmapLister:      configmapInformer.Lister(),
		secretLister:         secretInformer.Lister(),
		serviceAccountLister: oidcServiceaccountInformer.Lister(),
	}
	impl := triggerreconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			ConfigStore:       featureStore,
			PromoteFilterFunc: filterTriggers(r.brokerLister),
		}
	})
	r.impl = impl

	r.sourceTracker = duck.NewListableTrackerFromTracker(ctx, source.Get, impl.Tracker)
	r.uriResolver = resolver.NewURIResolverFromTracker(ctx, impl.Tracker)

	triggerInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: filterTriggers(r.brokerLister),
		Handler:    controller.HandleAll(impl.Enqueue),
	})

	// Filter Brokers and enqueue associated Triggers
	brokerFilter := pkgreconciler.AnnotationFilterFunc(brokerreconciler.ClassAnnotationKey, apiseventing.MTChannelBrokerClassValue, false /*allowUnset*/)
	brokerInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: brokerFilter,
		Handler: controller.HandleAll(func(obj interface{}) {
			if broker, ok := obj.(*eventing.Broker); ok {
				for _, t := range getTriggersForBroker(logger, triggerLister, broker) {
					impl.Enqueue(t)
				}
			}
		}),
	})

	// Reconcile Trigger when my Subscription changes
	subscriptionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&eventing.Trigger{}),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	// Reconciler Trigger when the OIDC service account changes
	oidcServiceaccountInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&eventing.Trigger{}),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}

// filterTriggers returns a function that returns true if the resource passed
// is a trigger pointing to a MTChannelBroker.
func filterTriggers(lister eventinglisters.BrokerLister) func(interface{}) bool {
	return func(obj interface{}) bool {
		trigger, ok := obj.(*eventing.Trigger)
		if !ok {
			return false
		}

		b, err := lister.Brokers(trigger.Namespace).Get(trigger.Spec.Broker)
		if err != nil {
			return false
		}

		value, ok := b.GetAnnotations()[apiseventing.BrokerClassKey]
		return ok && value == apiseventing.MTChannelBrokerClassValue
	}
}

// getTriggersForBroker makes sure the object passed in is a Broker, and gets all
// the Triggers belonging to it. As there is no way to return failures in the
// Informers EventHandler, errors are logged, and an empty array is returned in case
// of failures.
func getTriggersForBroker(logger *zap.SugaredLogger, triggerLister eventinglisters.TriggerLister, broker *eventing.Broker) []*eventing.Trigger {
	r := make([]*eventing.Trigger, 0)
	selector := labels.SelectorFromSet(map[string]string{apiseventing.BrokerLabelKey: broker.Name})
	triggers, err := triggerLister.Triggers(broker.Namespace).List(selector)
	if err != nil {
		logger.Warn("Failed to list triggers", zap.Any("broker", broker), zap.Error(err))
		return r
	}
	return append(r, triggers...)
}
