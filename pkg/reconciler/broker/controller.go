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
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/service"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1alpha1/channelable"
	brokerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/broker"
	subscriptioninformer "knative.dev/eventing/pkg/client/injection/informers/messaging/v1alpha1/subscription"
	servinginformer "knative.dev/eventing/pkg/client/injection/serving/informers/v1/service"
	kubeservice "knative.dev/eventing/pkg/reconciler/service/kube"
	servingservice "knative.dev/eventing/pkg/reconciler/service/serving"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	serviceinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/service"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	servingclient "knative.dev/serving/pkg/client/injection/client"
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
	ResourceFlavor        string `envconfig:"BROKER_RESOURCE_FLAVOR"`
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
	subscriptionInformer := subscriptioninformer.Get(ctx)
	serviceInformer := serviceinformer.Get(ctx)
	servingInformer := servinginformer.Get(ctx)

	if env.ResourceFlavor == service.ServingFlavor && servingInformer.IsEmpty() {
		log.Fatalf(`BROKER_RESOURCE_FLAVOR is set to %q but %v was not available`, service.ServingFlavor, servingv1.SchemeGroupVersion)
	}

	var svcReconciler service.Reconciler
	if env.ResourceFlavor == service.ServingFlavor {
		svcReconciler = &servingservice.ServiceReconciler{
			ServingClientSet: servingclient.Get(ctx),
			ServingLister:    servingInformer.GetInternal().Lister(),
		}
	} else {
		svcReconciler = &kubeservice.ServiceReconciler{
			KubeClientSet:    kubeclient.Get(ctx),
			DeploymentLister: deploymentInformer.Lister(),
			ServiceLister:    serviceInformer.Lister(),
		}
	}

	r := &Reconciler{
		Base:                      reconciler.NewBase(ctx, controllerAgentName, cmw),
		brokerLister:              brokerInformer.Lister(),
		subscriptionLister:        subscriptionInformer.Lister(),
		ingressImage:              env.IngressImage,
		ingressServiceAccountName: env.IngressServiceAccount,
		filterImage:               env.FilterImage,
		filterServiceAccountName:  env.FilterServiceAccount,
		svcReconciler:             svcReconciler,
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")

	r.channelableTracker = duck.NewListableTracker(ctx, channelable.Get, impl.EnqueueKey, controller.GetTrackerLease(ctx))

	brokerInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	serviceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Broker")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Broker")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	if !servingInformer.IsEmpty() {
		servingInformer.GetInternal().Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Broker")),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})
	}

	return impl
}
