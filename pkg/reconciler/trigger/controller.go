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
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventinginformers "github.com/knative/eventing/pkg/client/informers/externalversions/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/duck"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/tracker"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
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
	opt reconciler.Options,
	triggerInformer eventinginformers.TriggerInformer,
	channelInformer eventinginformers.ChannelInformer,
	subscriptionInformer eventinginformers.SubscriptionInformer,
	brokerInformer eventinginformers.BrokerInformer,
	serviceInformer corev1informers.ServiceInformer,
) *controller.Impl {

	r := &Reconciler{
		Base:               reconciler.NewBase(opt, controllerAgentName),
		triggerLister:      triggerInformer.Lister(),
		channelLister:      channelInformer.Lister(),
		subscriptionLister: subscriptionInformer.Lister(),
		brokerLister:       brokerInformer.Lister(),
		serviceLister:      serviceInformer.Lister(),
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")
	triggerInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	r.addressableInformer = duck.NewAddressableInformer(opt)

	// Tracker is used to notify us that a Trigger's Broker has changed so that
	// we can reconcile.
	r.tracker = tracker.New(impl.EnqueueKey, opt.GetTrackerLease())
	brokerInformer.Informer().AddEventHandler(controller.HandleAll(
		// Call the tracker's OnChanged method, but we've seen the objects
		// coming through this path missing TypeMeta, so ensure it is properly
		// populated.
		controller.EnsureTypeMeta(
			r.tracker.OnChanged,
			v1alpha1.SchemeGroupVersion.WithKind("Broker"),
		),
	))

	subscriptionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Trigger")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})
	return impl
}
