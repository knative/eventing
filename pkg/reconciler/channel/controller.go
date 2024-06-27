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

	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/auth"

	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/pkg/logging"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/clients/dynamicclient"

	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1/channelable"
	"knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventpolicy"
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
	eventPolicyInformer := eventpolicy.Get(ctx)

	r := &Reconciler{
		dynamicClientSet:  dynamicclient.Get(ctx),
		channelLister:     channelInformer.Lister(),
		eventPolicyLister: eventPolicyInformer.Lister(),
		eventingClientSet: eventingclient.Get(ctx),
	}

	var globalResync func()
	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"), func(name string, value interface{}) {
		if globalResync != nil {
			globalResync()
		}
	})
	featureStore.WatchConfigs(cmw)

	impl := channelreconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			ConfigStore: featureStore,
		}
	})

	r.channelableTracker = duck.NewListableTrackerFromTracker(ctx, channelable.Get, impl.Tracker)

	globalResync = func() {
		impl.GlobalResync(channelInformer.Informer())
	}

	channelInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	channelGK := messagingv1.SchemeGroupVersion.WithKind("Channel").GroupKind()

	// Enqueue the Channel, if we have an EventPolicy which was referencing
	// or got updated and now is referencing the Channel
	eventPolicyInformer.Informer().AddEventHandler(auth.EventPolicyEventHandler(channelInformer.Informer().GetIndexer(), channelGK, impl.EnqueueKey))

	return impl
}
