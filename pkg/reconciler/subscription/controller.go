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
	eventinginformers "github.com/knative/eventing/pkg/client/informers/externalversions/eventing/v1alpha1"
	eventingduck "github.com/knative/eventing/pkg/duck"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/tracker"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions/apiextensions/v1beta1"
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
	opt reconciler.Options,
	subscriptionInformer eventinginformers.SubscriptionInformer,
	addressableInformer eventingduck.AddressableInformer,
	customResourceDefinitionInformer apiextensionsinformers.CustomResourceDefinitionInformer,
) *controller.Impl {

	r := &Reconciler{
		Base:                           reconciler.NewBase(opt, controllerAgentName),
		subscriptionLister:             subscriptionInformer.Lister(),
		customResourceDefinitionLister: customResourceDefinitionInformer.Lister(),
		addressableInformer:            addressableInformer,
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")
	subscriptionInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Tracker is used to notify us when the resources Subscription depends on change, so that the
	// Subscription needs to reconcile again.
	r.tracker = tracker.New(impl.EnqueueKey, opt.GetTrackerLease())

	return impl
}
