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

package broker

import (
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventinginformers "github.com/knative/eventing/pkg/client/informers/externalversions/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/duck"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/tracker"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Brokers"
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "broker-controller"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	opt reconciler.Options,
	brokerInformer eventinginformers.BrokerInformer,
	subscriptionInformer eventinginformers.SubscriptionInformer,
	channelInformer eventinginformers.ChannelInformer,
	serviceInformer corev1informers.ServiceInformer,
	deploymentInformer appsv1informers.DeploymentInformer,
	addressableInformer duck.AddressableInformer,
	args ReconcilerArgs,
) *controller.Impl {

	r := &Reconciler{
		Base:                      reconciler.NewBase(opt, controllerAgentName),
		brokerLister:              brokerInformer.Lister(),
		channelLister:             channelInformer.Lister(),
		serviceLister:             serviceInformer.Lister(),
		deploymentLister:          deploymentInformer.Lister(),
		subscriptionLister:        subscriptionInformer.Lister(),
		addressableInformer:       addressableInformer,
		ingressImage:              args.IngressImage,
		ingressServiceAccountName: args.IngressServiceAccountName,
		filterImage:               args.FilterImage,
		filterServiceAccountName:  args.FilterServiceAccountName,
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")

	r.tracker = tracker.New(impl.EnqueueKey, opt.GetTrackerLease())

	brokerInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	channelInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Broker")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	serviceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Broker")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Broker")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
