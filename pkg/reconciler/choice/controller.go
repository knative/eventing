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

package choice

import (
	"context"

	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	"knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/subscription"
	"knative.dev/eventing/pkg/client/injection/informers/messaging/v1alpha1/choice"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Choices"
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "choice-controller"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	choiceInformer := choice.Get(ctx)
	subscriptionInformer := subscription.Get(ctx)
	resourceInformer := duck.NewResourceInformer(ctx)

	r := &Reconciler{
		Base:               reconciler.NewBase(ctx, controllerAgentName, cmw),
		choiceLister:       choiceInformer.Lister(),
		subscriptionLister: subscriptionInformer.Lister(),
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")

	r.resourceTracker = resourceInformer.NewTracker(impl.EnqueueKey, controller.GetTrackerLease(ctx))
	choiceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Register handler for Subscriptions that are owned by Choice, so that
	// we get notified if they change.
	subscriptionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Choice")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
