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

package parallel

import (
	"context"

	"knative.dev/eventing/pkg/auth"

	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/feature"
	v1 "knative.dev/eventing/pkg/apis/flows/v1"
	"knative.dev/eventing/pkg/duck"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	serviceaccountinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/filtered"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/logging"

	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1/channelable"
	"knative.dev/eventing/pkg/client/injection/informers/flows/v1/parallel"
	"knative.dev/eventing/pkg/client/injection/informers/messaging/v1/subscription"
	parallelreconciler "knative.dev/eventing/pkg/client/injection/reconciler/flows/v1/parallel"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	parallelInformer := parallel.Get(ctx)
	subscriptionInformer := subscription.Get(ctx)
	oidcServiceaccountInformer := serviceaccountinformer.Get(ctx, auth.OIDCLabelSelector)

	var globalResync func(obj interface{})
	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"), func(name string, value interface{}) {
		if globalResync != nil {
			globalResync(nil)
		}
	})
	featureStore.WatchConfigs(cmw)

	r := &Reconciler{
		parallelLister:       parallelInformer.Lister(),
		subscriptionLister:   subscriptionInformer.Lister(),
		serviceAccountLister: oidcServiceaccountInformer.Lister(),
		kubeclient:           kubeclient.Get(ctx),
		dynamicClientSet:     dynamicclient.Get(ctx),
		eventingClientSet:    eventingclient.Get(ctx),
	}
	impl := parallelreconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			ConfigStore: featureStore,
		}
	})

	globalResync = func(_ interface{}) {
		impl.GlobalResync(parallelInformer.Informer())
	}

	r.channelableTracker = duck.NewListableTrackerFromTracker(ctx, channelable.Get, impl.Tracker)
	parallelInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Register handler for Subscriptions that are owned by Parallel, so that
	// we get notified if they change.
	subscriptionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&v1.Parallel{}),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})
	// Reconcile Parallel when the OIDC service account changes
	oidcServiceaccountInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&v1.Parallel{}),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
