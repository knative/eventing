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

package parallel

import (
	"context"

	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/flows/v1beta1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/logging"

	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1beta1/channelable"
	"knative.dev/eventing/pkg/client/injection/informers/flows/v1beta1/parallel"
	"knative.dev/eventing/pkg/client/injection/informers/messaging/v1beta1/subscription"
	parallelreconciler "knative.dev/eventing/pkg/client/injection/reconciler/flows/v1beta1/parallel"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	parallelInformer := parallel.Get(ctx)
	subscriptionInformer := subscription.Get(ctx)

	r := &Reconciler{
		parallelLister:     parallelInformer.Lister(),
		subscriptionLister: subscriptionInformer.Lister(),
		dynamicClientSet:   dynamicclient.Get(ctx),
		eventingClientSet:  eventingclient.Get(ctx),
	}
	impl := parallelreconciler.NewImpl(ctx, r)

	logging.FromContext(ctx).Info("Setting up event handlers")

	r.channelableTracker = duck.NewListableTracker(ctx, channelable.Get, impl.EnqueueKey, controller.GetTrackerLease(ctx))
	parallelInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Register handler for Subscriptions that are owned by Parallel, so that
	// we get notified if they change.
	subscriptionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterGroupKind(v1beta1.Kind("Parallel")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
