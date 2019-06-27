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
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/duck"
	"github.com/knative/eventing/pkg/reconciler"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	brokerinformer "github.com/knative/eventing/pkg/client/injection/informers/eventing/v1alpha1/broker"
	channelinformer "github.com/knative/eventing/pkg/client/injection/informers/eventing/v1alpha1/channel"
	subscriptioninformer "github.com/knative/eventing/pkg/client/injection/informers/eventing/v1alpha1/subscription"
	deploymentinformer "knative.dev/pkg/injection/informers/kubeinformers/appsv1/deployment"
	serviceinformer "knative.dev/pkg/injection/informers/kubeinformers/corev1/service"
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
	channelInformer := channelinformer.Get(ctx)
	subscriptionInformer := subscriptioninformer.Get(ctx)
	serviceInformer := serviceinformer.Get(ctx)
	addressableInformer := duck.NewAddressableInformer(ctx)

	r := &Reconciler{
		Base:                      reconciler.NewBase(ctx, controllerAgentName, cmw),
		brokerLister:              brokerInformer.Lister(),
		channelLister:             channelInformer.Lister(),
		serviceLister:             serviceInformer.Lister(),
		deploymentLister:          deploymentInformer.Lister(),
		subscriptionLister:        subscriptionInformer.Lister(),
		ingressImage:              env.IngressImage,
		ingressServiceAccountName: env.IngressServiceAccount,
		filterImage:               env.FilterImage,
		filterServiceAccountName:  env.FilterServiceAccount,
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")

	r.addressableTracker = addressableInformer.NewTracker(impl.EnqueueKey, controller.GetTrackerLease(ctx))

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
