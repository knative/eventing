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

package subscription

import (
	"context"

	"knative.dev/eventing/pkg/auth"

	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/pkg/client/injection/apiextensions/informers/apiextensions/v1/customresourcedefinition"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kref"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"

	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1/channelable"
	"knative.dev/eventing/pkg/client/injection/informers/messaging/v1/channel"
	"knative.dev/eventing/pkg/client/injection/informers/messaging/v1/subscription"
	subscriptionreconciler "knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1/subscription"
	"knative.dev/eventing/pkg/duck"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	serviceaccountinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/filtered"
	"knative.dev/pkg/injection/clients/dynamicclient"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	subscriptionInformer := subscription.Get(ctx)
	channelInformer := channel.Get(ctx)
	oidcServiceaccountInformer := serviceaccountinformer.Get(ctx, auth.OIDCLabelSelector)

	var globalResync func(obj interface{})

	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"), func(name string, value interface{}) {
		if globalResync != nil {
			globalResync(nil)
		}
	})
	featureStore.WatchConfigs(cmw)

	r := &Reconciler{
		dynamicClientSet:     dynamicclient.Get(ctx),
		kubeclient:           kubeclient.Get(ctx),
		kreferenceResolver:   kref.NewKReferenceResolver(customresourcedefinition.Get(ctx).Lister()),
		subscriptionLister:   subscriptionInformer.Lister(),
		channelLister:        channelInformer.Lister(),
		serviceAccountLister: oidcServiceaccountInformer.Lister(),
	}
	impl := subscriptionreconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			ConfigStore: featureStore,
		}
	})

	globalResync = func(_ interface{}) {
		impl.GlobalResync(subscriptionInformer.Informer())
	}

	subscriptionInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Trackers used to notify us when the resources Subscription depends on change, so that the
	// Subscription needs to reconcile again.
	r.channelableTracker = duck.NewListableTrackerFromTracker(ctx, channelable.Get, impl.Tracker)
	r.destinationResolver = resolver.NewURIResolverFromTracker(ctx, impl.Tracker)

	// Track changes to Channels.
	r.tracker = impl.Tracker
	channelInformer.Informer().AddEventHandler(controller.HandleAll(
		// Call the tracker's OnChanged method, but we've seen the objects
		// coming through this path missing TypeMeta, so ensure it is properly
		// populated.
		controller.EnsureTypeMeta(
			r.tracker.OnChanged,
			messagingv1.SchemeGroupVersion.WithKind("Channel"),
		),
	))

	// Reconciler Subscription when the OIDC service account changes
	oidcServiceaccountInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&messagingv1.Subscription{}),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})
	return impl
}
