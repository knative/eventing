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

package channel

import (
	"context"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1/channelable"
	channelinformer "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/channel"
	channelreconciler "knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1/channel"
	"knative.dev/eventing/pkg/duck"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	channelInformer := channelinformer.Get(ctx)

	r := &Reconciler{
		dynamicClientSet: dynamicclient.Get(ctx),
		channelLister:    channelInformer.Lister(),
	}
	impl := channelreconciler.NewImpl(ctx, r)

	r.channelableTracker = duck.NewListableTracker(ctx, channelable.Get, impl.EnqueueKey, controller.GetTrackerLease(ctx))

	logging.FromContext(ctx).Info("Setting up event handlers")

	channelInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	return impl
}
