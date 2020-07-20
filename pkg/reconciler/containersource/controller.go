/*
Copyright 2020 The Knative Authors

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

package containersource

import (
	"context"

	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/sources/v1beta1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	containersourceinformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1beta1/containersource"
	sinkbindinginformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1beta1/sinkbinding"
	v1beta1containersource "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1beta1/containersource"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// NewController creates a Reconciler for ContainerSource and returns the result of NewImpl.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	kubeClient := kubeclient.Get(ctx)
	eventingClient := eventingclient.Get(ctx)
	containersourceInformer := containersourceinformer.Get(ctx)
	sinkbindingInformer := sinkbindinginformer.Get(ctx)
	deploymentInformer := deploymentinformer.Get(ctx)

	r := &Reconciler{
		kubeClientSet:         kubeClient,
		eventingClientSet:     eventingClient,
		containerSourceLister: containersourceInformer.Lister(),
		deploymentLister:      deploymentInformer.Lister(),
		sinkBindingLister:     sinkbindingInformer.Lister(),
	}
	impl := v1beta1containersource.NewImpl(ctx, r)

	logging.FromContext(ctx).Info("Setting up event handlers.")
	containersourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGVK(v1beta1.SchemeGroupVersion.WithKind("ContainerSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	sinkbindingInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(v1beta1.Kind("ContainerSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
