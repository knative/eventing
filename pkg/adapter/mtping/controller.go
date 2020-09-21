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

	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/eventing/pkg/apis/sources/v1beta1"
	pingsourceinformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1beta1/pingsource"
	pingsourcereconciler "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1beta1/pingsource"
)

// TODO: code generation

// MTAdapter is the interface the multi-tenant PingSource adapter must implement
type MTAdapter interface {
	// Update is called when the source is ready and when the specification and/or status has changed.
	Update(ctx context.Context, source *v1beta1.PingSource)

	// Remove is called when the source has been deleted.
	Remove(ctx context.Context, source *v1beta1.PingSource)
}

// NewController initializes the controller. This is called by the shared adapter Main
// Registers event handlers to enqueue events.
func NewController(ctx context.Context, adapter adapter.Adapter) *controller.Impl {
	mtadapter, ok := adapter.(MTAdapter)
	if !ok {
		logging.FromContext(ctx).Fatal("Multi-tenant adapters must implement the MTAdapter interface")
	}

	r := &Reconciler{mtadapter}

	// TODO: need pkg#1683
	// lister := pingsourceinformer.Get(ctx).Lister()
	//opts := func(impl *controller.Impl) controller.Options {
	//	return controller.Options{
	//		DemoteFunc: func(b reconciler.Bucket) {
	//			all, _ := lister.List(labels.Everything())
	//			// TODO: demote with error
	//			//if err != nil {
	//			//	return err
	//			//}
	//			for _, elt := range all {
	//				mtadapter.Remove(ctx, elt)
	//			}
	//		},
	//	}
	//}

	impl := pingsourcereconciler.NewImpl(ctx, r)

	logging.FromContext(ctx).Info("Setting up event handlers")
	pingsourceinformer.Get(ctx).Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))
	return impl
}
