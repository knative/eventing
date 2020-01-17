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

package controller

import (
	"context"

	"github.com/kelseyhightower/envconfig"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	"knative.dev/eventing/pkg/client/injection/informers/messaging/v1alpha1/inmemorychannel"
	"knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/endpoints"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/service"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount"
	"knative.dev/pkg/client/injection/kube/informers/rbac/v1/rolebinding"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "InMemoryChannels"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "in-memory-channel-controller"

	// TODO: this should be passed in on the env.
	dispatcherName = "imc-dispatcher"
)

var (
	serviceAccountGVK = corev1.SchemeGroupVersion.WithKind("ServiceAccount")
	roleBindingGVK    = rbacv1.SchemeGroupVersion.WithKind("RoleBinding")
)

type envConfig struct {
	Image string `envconfig:"DISPATCHER_IMAGE" required:"true"`
}

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	inmemorychannelInformer := inmemorychannel.Get(ctx)
	deploymentInformer := deployment.Get(ctx)
	serviceInformer := service.Get(ctx)
	endpointsInformer := endpoints.Get(ctx)
	serviceAccountInformer := serviceaccount.Get(ctx)
	roleBindingInformer := rolebinding.Get(ctx)

	r := &Reconciler{
		Base: reconciler.NewBase(ctx, controllerAgentName, cmw),

		inmemorychannelLister:   inmemorychannelInformer.Lister(),
		inmemorychannelInformer: inmemorychannelInformer.Informer(),
		deploymentLister:        deploymentInformer.Lister(),
		serviceLister:           serviceInformer.Lister(),
		endpointsLister:         endpointsInformer.Lister(),
		serviceAccountLister:    serviceAccountInformer.Lister(),
		roleBindingLister:       roleBindingInformer.Lister(),
	}

	env := &envConfig{}
	if err := envconfig.Process("", env); err != nil {
		r.Logger.Panicf("unable to process in-memory channel's required environment variables: %v", err)
	}
	r.dispatcherImage = env.Image

	r.impl = controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")
	inmemorychannelInformer.Informer().AddEventHandler(controller.HandleAll(r.impl.Enqueue))

	// Set up watches for dispatcher resources we care about, since any changes to these
	// resources will affect our Channels. So, set up a watch here, that will cause
	// a global Resync for all the channels to take stock of their health when these change.
	deploymentInformer.Informer().AddEventHandler(r)
	serviceInformer.Informer().AddEventHandler(r)
	endpointsInformer.Informer().AddEventHandler(r)
	serviceAccountInformer.Informer().AddEventHandler(r)
	roleBindingInformer.Informer().AddEventHandler(r)

	return r.impl
}
