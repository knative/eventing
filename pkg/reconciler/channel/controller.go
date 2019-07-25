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
	"github.com/knative/eventing/pkg/duck"

	"github.com/knative/eventing/pkg/reconciler"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	channelinformer "github.com/knative/eventing/pkg/client/injection/informers/messaging/v1alpha1/channel"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Channels"
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "ch-default-controller"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	channelInformer := channelinformer.Get(ctx)
	resourceInformer := duck.NewResourceInformer(ctx)

	r := &Reconciler{
		Base:          reconciler.NewBase(ctx, controllerAgentName, cmw),
		channelLister: channelInformer.Lister(),
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName)

	r.resourceTracker = resourceInformer.NewTracker(impl.EnqueueKey, controller.GetTrackerLease(ctx))

	r.Logger.Info("Setting up event handlers")
	channelInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	return impl
}
