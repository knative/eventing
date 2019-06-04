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

package eventtype

import (
	"github.com/knative/pkg/tracker"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventinginformers "github.com/knative/eventing/pkg/client/informers/externalversions/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/controller"
)

const (
	// ReconcilerName is the name of the reconciler.
	ReconcilerName = "EventTypes"
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "eventtype-controller"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	opt reconciler.Options,
	eventTypeInformer eventinginformers.EventTypeInformer,
	brokerInformer eventinginformers.BrokerInformer,
) *controller.Impl {

	r := &Reconciler{
		Base:            reconciler.NewBase(opt, controllerAgentName),
		eventTypeLister: eventTypeInformer.Lister(),
		brokerLister:    brokerInformer.Lister(),
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")
	eventTypeInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Tracker is used to notify us that a EventType's Broker has changed so that
	// we can reconcile.
	r.tracker = tracker.New(impl.EnqueueKey, opt.GetTrackerLease())
	brokerInformer.Informer().AddEventHandler(controller.HandleAll(
		controller.EnsureTypeMeta(
			r.tracker.OnChanged,
			v1alpha1.SchemeGroupVersion.WithKind("Broker"),
		),
	))

	return impl
}
