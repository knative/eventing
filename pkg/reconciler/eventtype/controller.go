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

package eventtype

import (
	"context"

	"knative.dev/eventing/pkg/resolver"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	eventtypeinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1beta2/eventtype"
	eventtypereconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1beta2/eventtype"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	eventTypeInformer := eventtypeinformer.Get(ctx)

	r := &Reconciler{}
	impl := eventtypereconciler.NewImpl(ctx, r)

	eventTypeInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Tracker is used to notify us that a EventType's Broker has changed so that
	// we can reconcile.
	r.kReferenceResolver = resolver.NewKReferenceResolverFromTracker(ctx, impl.Tracker)

	return impl
}
