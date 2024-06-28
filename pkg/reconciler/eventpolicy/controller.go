/*
Copyright 2024 The Knative Authors

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

package eventpolicy

import (
	"context"

	eventpolicyinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventpolicy"
	eventpolicyreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1alpha1/eventpolicy"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/resolver"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	// Access informers
	eventPolicyInformer := eventpolicyinformer.Get(ctx)

	r := &Reconciler{}
	impl := eventpolicyreconciler.NewImpl(ctx, r)

	r.authResolver = resolver.NewAuthenticatableResolverFromTracker(ctx, impl.Tracker)

	// Set up event handlers
	eventPolicyInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	return impl
}
