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

	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"

	crd "knative.dev/pkg/client/injection/apiextensions/informers/apiextensions/v1beta1/customresourcedefinition"

	"knative.dev/eventing/pkg/client/injection/informers/messaging/v1alpha1/subscription"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Subscriptions"
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "subscription-controller"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	subscriptionInformer := subscription.Get(ctx)
	customResourceDefinitionInformer := crd.Get(ctx)

	r := &Reconciler{
		Base:                           reconciler.NewBase(ctx, controllerAgentName, cmw),
		subscriptionLister:             subscriptionInformer.Lister(),
		customResourceDefinitionLister: customResourceDefinitionInformer.Lister(),
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")
	subscriptionInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Trackers used to notify us when the resources Subscription depends on change, so that the
	// Subscription needs to reconcile again.
	r.addressableTracker = duck.NewListableTracker(ctx, &duckv1alpha1.AddressableType{}, impl.EnqueueKey, controller.GetTrackerLease(ctx))
	r.channelableTracker = duck.NewListableTracker(ctx, &eventingduckv1alpha1.Channelable{}, impl.EnqueueKey, controller.GetTrackerLease(ctx))

	return impl
}
