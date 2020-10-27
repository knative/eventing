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

package pingsource

import (
	"context"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/tools/cache"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/system"
	"knative.dev/pkg/tracker"

	"knative.dev/eventing/pkg/adapter/v2"
	pingsourceinformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1beta1/pingsource"
	pingsourcereconciler "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1beta1/pingsource"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
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

	cc := leaderElectionConfig.GetComponentConfig(mtcomponent)
	leConfig, err := adapter.LeaderElectionComponentConfigToJSON(&cc)
	if err != nil {
		logger.Fatalw("Error converting leader election configuration to JSON", zap.Error(err))
	}

	// Configure the reconciler

	deploymentInformer := deploymentinformer.Get(ctx)
	pingSourceInformer := pingsourceinformer.Get(ctx)

	r := &Reconciler{
		kubeClientSet:    kubeclient.Get(ctx),
		pingLister:       pingSourceInformer.Lister(),
		deploymentLister: deploymentInformer.Lister(),
		leConfig:         leConfig,
		loggingContext:   ctx,
		configs:          reconcilersource.WatchConfigurations(ctx, component, cmw),
	}

	impl := pingsourcereconciler.NewImpl(ctx, r)

	r.sinkResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	logger.Info("Setting up event handlers")
	pingSourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Tracker is used to notify us that the pingsource-mt-adapter Deployment has changed so that
	// we can reconcile PingSources that depends on it
	r.tracker = tracker.New(impl.EnqueueKey, controller.GetTrackerLease(ctx))

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(system.Namespace(), mtadapterName),
		Handler: controller.HandleAll(
			controller.EnsureTypeMeta(
				r.tracker.OnChanged,
				appsv1.SchemeGroupVersion.WithKind("Deployment"),
			)),
	})

	return impl
}
