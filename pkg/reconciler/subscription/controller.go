/*
Copyright 2019 The Knative Authors

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

package subscription

import (
	"context"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/tracker"

	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1alpha1/channelablecombined"
	"knative.dev/eventing/pkg/client/injection/informers/messaging/v1/channel"
	"knative.dev/eventing/pkg/client/injection/informers/messaging/v1/subscription"
	subscriptionreconciler "knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1/subscription"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/pkg/injection/clients/dynamicclient"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	subscriptionInformer := subscription.Get(ctx)
	channelInformer := channel.Get(ctx)

	r := &Reconciler{
		dynamicClientSet:   dynamicclient.Get(ctx),
		subscriptionLister: subscriptionInformer.Lister(),
		channelLister:      channelInformer.Lister(),
	}
	impl := subscriptionreconciler.NewImpl(ctx, r)

	logging.FromContext(ctx).Info("Setting up event handlers")
	subscriptionInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Trackers used to notify us when the resources Subscription depends on change, so that the
	// Subscription needs to reconcile again.
	r.channelableTracker = duck.NewListableTracker(ctx, channelablecombined.Get, impl.EnqueueKey, controller.GetTrackerLease(ctx))
	r.destinationResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	// Track changes to Channels.
	r.tracker = tracker.New(impl.EnqueueKey, controller.GetTrackerLease(ctx))
	channelInformer.Informer().AddEventHandler(controller.HandleAll(
		// Call the tracker's OnChanged method, but we've seen the objects
		// coming through this path missing TypeMeta, so ensure it is properly
		// populated.
		controller.EnsureTypeMeta(
			r.tracker.OnChanged,
			messagingv1.SchemeGroupVersion.WithKind("Channel"),
		),
	))

	return impl
}
