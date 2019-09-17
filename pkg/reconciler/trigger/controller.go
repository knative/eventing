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

package trigger

import (
	"context"

	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler"
	apisduck "knative.dev/pkg/apis/duck"

	"knative.dev/pkg/client/injection/kube/informers/core/v1/service"

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/broker"
	"knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/trigger"
	"knative.dev/eventing/pkg/client/injection/informers/messaging/v1alpha1/subscription"
	"knative.dev/pkg/injection/clients/dynamicclient"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Triggers"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "trigger-controller"
)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	triggerInformer := trigger.Get(ctx)
	subscriptionInformer := subscription.Get(ctx)
	brokerInformer := broker.Get(ctx)
	serviceInformer := service.Get(ctx)
	resourceInformer := duck.NewResourceInformer(ctx)

	r := &Reconciler{
		Base:               reconciler.NewBase(ctx, controllerAgentName, cmw),
		triggerLister:      triggerInformer.Lister(),
		subscriptionLister: subscriptionInformer.Lister(),
		brokerLister:       brokerInformer.Lister(),
		serviceLister:      serviceInformer.Lister(),
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")
	triggerInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Tracker is used to notify us that a Trigger's Broker has changed so that
	// we can reconcile.
	r.resourceTracker = resourceInformer.NewTracker(impl.EnqueueKey, controller.GetTrackerLease(ctx))
	r.kresourceInformerFactory = KResourceTypedInformerFactory(ctx)

	subscriptionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Trigger")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})
	return impl
}

func KResourceTypedInformerFactory(ctx context.Context) apisduck.InformerFactory {
	return &apisduck.TypedInformerFactory{
		Client:       dynamicclient.Get(ctx),
		Type:         &duckv1alpha1.KResource{},
		ResyncPeriod: controller.GetResyncPeriod(ctx),
		StopChannel:  ctx.Done(),
	}
}
