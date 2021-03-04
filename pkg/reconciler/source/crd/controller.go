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

package crd

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/sources"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"

	crdinfomer "knative.dev/pkg/client/injection/apiextensions/informers/apiextensions/v1/customresourcedefinition"
	crdreconciler "knative.dev/pkg/client/injection/apiextensions/reconciler/apiextensions/v1/customresourcedefinition"
)

const (
	// ReconcilerName is the name of the reconciler.
	ReconcilerName = "SourceCRDs"
)

// NewController creates a Reconciler for Sources' CRDs and returns the result of NewImpl.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)
	crdInformer := crdinfomer.Get(ctx)

	r := &Reconciler{
		ogctx:       ctx,
		ogcmw:       cmw,
		controllers: make(map[schema.GroupVersionResource]runningController),
	}
	impl := crdreconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			AgentName: ReconcilerName,
		}
	})

	logger.Info("Setting up event handlers")
	crdInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: pkgreconciler.LabelFilterFunc(sources.SourceDuckLabelKey, sources.SourceDuckLabelValue, false),
		Handler:    controller.HandleAll(impl.Enqueue),
	})

	return impl
}
