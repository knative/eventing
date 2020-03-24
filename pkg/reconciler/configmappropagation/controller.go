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

package configmappropagation

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracker"

	configmappropagationinformer "knative.dev/eventing/pkg/client/injection/informers/configs/v1alpha1/configmappropagation"
	"knative.dev/eventing/pkg/client/injection/reconciler/configs/v1alpha1/configmappropagation"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap"
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
		kubeClientSet:              kubeclient.Get(ctx),
		configMapPropagationLister: configMapPropagationInformer.Lister(),
		configMapLister:            configMapInformer.Lister(),
	}

	impl := configmappropagation.NewImpl(ctx, r)

	logging.FromContext(ctx).Info("Setting up event handlers")
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
