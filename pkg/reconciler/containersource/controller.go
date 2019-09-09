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

package containersource

import (
	"context"

	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	containersourceinformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1alpha1/containersource"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "ContainerSources"
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "container-source-controller"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	containerSourceInformer := containersourceinformer.Get(ctx)
	deploymentInformer := deploymentinformer.Get(ctx)

	r := &Reconciler{
		Base:                  reconciler.NewBase(ctx, controllerAgentName, cmw),
		containerSourceLister: containerSourceInformer.Lister(),
		deploymentLister:      deploymentInformer.Lister(),
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName)
	r.sinkReconciler = duck.NewSinkReconciler(ctx, impl.EnqueueKey)

	r.Logger.Info("Setting up event handlers")
	containerSourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("ContainerSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
