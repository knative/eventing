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

package configmappropagation

import (
	"context"

	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/tracker"

	corev1 "k8s.io/api/core/v1"
	configmappropagationinformer "knative.dev/eventing/pkg/client/injection/informers/configs/v1alpha1/configmappropagation"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap"
)

const (
	// ReconcilerName is the name of the reconciler.
	ReconcilerName = "ConfigMapPropagation"
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "configmappropagation-controller"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	configMapPropagationInformer := configmappropagationinformer.Get(ctx)
	configMapInformer := configmapinformer.Get(ctx)

	r := &Reconciler{
		Base:                       reconciler.NewBase(ctx, controllerAgentName, cmw),
		configMapPropagationLister: configMapPropagationInformer.Lister(),
		configMapLister:            configMapInformer.Lister(),
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")
	configMapPropagationInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Tracker is used to notify us that a ConfigMap has changed so that
	// we can reconcile.
	r.tracker = tracker.New(impl.EnqueueKey, controller.GetTrackerLease(ctx))
	configMapInformer.Informer().AddEventHandler(controller.HandleAll(
		controller.EnsureTypeMeta(
			r.tracker.OnChanged,
			corev1.SchemeGroupVersion.WithKind("ConfigMap"),
		),
	))

	return impl
}
