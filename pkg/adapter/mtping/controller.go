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

package mtping

import (
	"context"

	"k8s.io/client-go/tools/cache"

	"knative.dev/eventing/pkg/adapter/v2"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	pingsourceinformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1/pingsource"
	pingsourcereconciler "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1/pingsource"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
)

// TODO: code generation

// MTAdapter is the interface the multi-tenant PingSource adapter must implement
type MTAdapter interface {
	// Update is called when the source is ready and when the specification and/or status has changed.
	Update(ctx context.Context, source *sourcesv1.PingSource)

	// Remove is called when the source has been deleted.
	Remove(source *sourcesv1.PingSource)

	// RemoveAll is called when the adapter stopped leading
	RemoveAll(ctx context.Context)
}

// NewController initializes the controller. This is called by the shared adapter Main
// Registers event handlers to enqueue events.
func NewController(ctx context.Context, adapter adapter.Adapter) *controller.Impl {
	mtadapter, ok := adapter.(MTAdapter)
	if !ok {
		logging.FromContext(ctx).Fatal("Multi-tenant adapters must implement the MTAdapter interface")
	}

	r := &Reconciler{mtadapter}

	impl := pingsourcereconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			SkipStatusUpdates: true,
			DemoteFunc: func(b reconciler.Bucket) {
				mtadapter.RemoveAll(ctx)
			},
		}
	})

	logging.FromContext(ctx).Info("Setting up event handlers")
	pingsourceinformer.Get(ctx).Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    impl.Enqueue,
			UpdateFunc: controller.PassNew(impl.Enqueue),
			DeleteFunc: r.deleteFunc,
		})
	return impl
}
