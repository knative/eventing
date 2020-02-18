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
	"context"
	"log"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1alpha1/channelable"
	brokerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/broker"
	triggerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/trigger"
	subscriptioninformer "knative.dev/eventing/pkg/client/injection/informers/messaging/v1alpha1/subscription"
	brokerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1alpha1/broker"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/client/injection/ducks/duck/v1/conditions"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	serviceinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/service"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Brokers"
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "broker-controller"
)

type envConfig struct {
	IngressImage          string `envconfig:"BROKER_INGRESS_IMAGE" required:"true"`
	IngressServiceAccount string `envconfig:"BROKER_INGRESS_SERVICE_ACCOUNT" required:"true"`
	FilterImage           string `envconfig:"BROKER_FILTER_IMAGE" required:"true"`
	FilterServiceAccount  string `envconfig:"BROKER_FILTER_SERVICE_ACCOUNT" required:"true"`
	BrokerClass           string `envconfig:"BROKER_CLASS"`
}

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env var", zap.Error(err))
	}

	deploymentInformer := deploymentinformer.Get(ctx)
	brokerInformer := brokerinformer.Get(ctx)
	triggerInformer := triggerinformer.Get(ctx)
	subscriptionInformer := subscriptioninformer.Get(ctx)
	serviceInformer := serviceinformer.Get(ctx)

	r := &Reconciler{
		Base:                      reconciler.NewBase(ctx, controllerAgentName, cmw),
		brokerLister:              brokerInformer.Lister(),
		serviceLister:             serviceInformer.Lister(),
		deploymentLister:          deploymentInformer.Lister(),
		subscriptionLister:        subscriptionInformer.Lister(),
		triggerLister:             triggerInformer.Lister(),
		ingressImage:              env.IngressImage,
		ingressServiceAccountName: env.IngressServiceAccount,
		filterImage:               env.FilterImage,
		filterServiceAccountName:  env.FilterServiceAccount,
		brokerClass:               env.BrokerClass,
	}
	impl := brokerreconciler.NewImpl(ctx, r)

	r.Logger.Info("Setting up event handlers")

	r.kresourceTracker = duck.NewListableTracker(ctx, conditions.Get, impl.EnqueueKey, controller.GetTrackerLease(ctx))
	r.channelableTracker = duck.NewListableTracker(ctx, channelable.Get, impl.EnqueueKey, controller.GetTrackerLease(ctx))
	r.addressableTracker = duck.NewListableTracker(ctx, addressable.Get, impl.EnqueueKey, controller.GetTrackerLease(ctx))
	r.uriResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	brokerInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	serviceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Broker")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Broker")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	// Reconcile Broker (which transitively reconciles the triggers), when Subscriptions
	// that I own are changed.
	subscriptionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Broker")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	// Reconcile trigger (by enqueuing the broker specified in the label) when subscriptions
	// of triggers change.
	subscriptionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: pkgreconciler.LabelExistsFilterFunc(eventing.BrokerLabelKey),
		Handler:    controller.HandleAll(impl.EnqueueLabelOfNamespaceScopedResource("" /*any namespace*/, eventing.BrokerLabelKey)),
	})

	triggerInformer.Informer().AddEventHandler(controller.HandleAll(
		func(obj interface{}) {
			if trigger, ok := obj.(*v1alpha1.Trigger); ok {
				impl.EnqueueKey(types.NamespacedName{Namespace: trigger.Namespace, Name: trigger.Spec.Broker})
			}
		},
	))

	return impl
}
