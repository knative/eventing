/*
Copyright 2021 The Knative Authors

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

package pingsource

import (
	"context"

	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/pkg/auth"

	serviceaccountinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/filtered"

	"go.uber.org/zap"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/tools/cache"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"

	"knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/eventing/pkg/apis/feature"
	pingsourceinformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1/pingsource"
	pingsourcereconciler "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1/pingsource"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
	"knative.dev/eventing/pkg/resolver"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	// Retrieve leader election config
	leaderElectionConfig, err := sharedmain.GetLeaderElectionConfig(ctx)
	if err != nil {
		logger.Fatalw("Error loading leader election configuration", zap.Error(err))
	}

	cc := leaderElectionConfig.GetComponentConfig(component)
	leConfig, err := adapter.LeaderElectionComponentConfigToJSON(&cc)
	if err != nil {
		logger.Fatalw("Error converting leader election configuration to JSON", zap.Error(err))
	}

	var globalResync func(obj interface{})

	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"), func(name string, value interface{}) {
		if globalResync != nil {
			globalResync(nil)
		}
	})
	featureStore.WatchConfigs(cmw)

	// Configure the reconciler

	deploymentInformer := deploymentinformer.Get(ctx)
	pingSourceInformer := pingsourceinformer.Get(ctx)
	oidcServiceaccountInformer := serviceaccountinformer.Get(ctx, auth.OIDCLabelSelector)

	r := &Reconciler{
		kubeClientSet:        kubeclient.Get(ctx),
		leConfig:             leConfig,
		configAcc:            reconcilersource.WatchConfigurations(ctx, component, cmw),
		serviceAccountLister: oidcServiceaccountInformer.Lister(),
	}

	impl := pingsourcereconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			ConfigStore: featureStore,
		}
	})

	globalResync = func(interface{}) {
		impl.GlobalResync(pingSourceInformer.Informer())
	}

	r.sinkResolver = resolver.NewURIResolver(ctx, cmw, impl.Tracker)

	pingSourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Tracker is used to notify us that the pingsource-mt-adapter Deployment has changed so that
	// we can reconcile PingSources that depend on it
	r.tracker = impl.Tracker

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(system.Namespace(), mtadapterName),
		Handler: controller.HandleAll(
			controller.EnsureTypeMeta(
				r.tracker.OnChanged,
				appsv1.SchemeGroupVersion.WithKind("Deployment"),
			)),
	})

	oidcServiceaccountInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&sourcesv1.PingSource{}),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
