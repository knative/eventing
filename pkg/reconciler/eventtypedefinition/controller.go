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

package eventtypedefinition

import (
	"context"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	eventtypedefinitioninformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventtypedefinition"
	eventtypedefinitionreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1alpha1/eventtypedefinition"
)

func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	eventtypedefinitioninformer := eventtypedefinitioninformer.Get(ctx)

	r := &Reconciler{
		eventTypeDefinitionLister: eventtypedefinitioninformer.Lister(),
	}
	impl := eventtypedefinitionreconciler.NewImpl(ctx, r)

	eventtypedefinitioninformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Tracker is used to notify us that a EventType's Broker has changed so that
	// we can reconcile.
	r.tracker = impl.Tracker

	return impl
}
