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

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	brokerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/broker"
	triggerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/trigger"
	subscriptioninformer "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/subscription"
	brokerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/broker"
	triggerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/trigger"
	v1 "knative.dev/eventing/pkg/client/listers/eventing/v1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/pkg/client/injection/ducks/duck/v1/source"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
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

	triggerLister := triggerInformer.Lister()
	r := &Reconciler{
		eventingClientSet:  eventingclient.Get(ctx),
		dynamicClientSet:   dynamicclient.Get(ctx),
		subscriptionLister: subscriptionInformer.Lister(),
		brokerLister:       brokerInformer.Lister(),
		triggerLister:      triggerLister,
		configmapLister:    configmapInformer.Lister(),
	}
	impl := triggerreconciler.NewImpl(ctx, r)
	r.impl = impl

	logger.Info("Setting up event handlers")

	r.sourceTracker = duck.NewListableTracker(ctx, source.Get, impl.EnqueueKey, controller.GetTrackerLease(ctx))
	r.uriResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	triggerInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Filter Brokers and enqueue associated Triggers
	brokerFilter := pkgreconciler.AnnotationFilterFunc(brokerreconciler.ClassAnnotationKey, eventing.MTChannelBrokerClassValue, false /*allowUnset*/)
	brokerInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: brokerFilter,
		Handler: controller.HandleAll(func(obj interface{}) {
			if broker, ok := obj.(*eventingv1.Broker); ok {
				for _, t := range getTriggersForBroker(logger, triggerLister, broker) {
					impl.Enqueue(t)
				}
			}
		}),
	})

	// Reconcile Trigger when my Subscription changes
	subscriptionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(eventingv1.Kind("Trigger")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}

// getTriggersForBroker makes sure the object passed in is a Broker, and gets all
// the Triggers belonging to it. As there is no way to return failures in the
// Informers EventHandler, errors are logged, and an empty array is returned in case
// of failures.
func getTriggersForBroker(logger *zap.SugaredLogger, triggerLister v1.TriggerLister, broker *eventingv1.Broker) []*eventingv1.Trigger {
	r := make([]*eventingv1.Trigger, 0)
	selector := labels.SelectorFromSet(map[string]string{eventing.BrokerLabelKey: broker.Name})
	triggers, err := triggerLister.Triggers(broker.Namespace).List(selector)
	if err != nil {
		logger.Warn("Failed to list triggers", zap.Any("broker", broker), zap.Error(err))
		return r
	}
	return append(r, triggers...)
}
