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
	"encoding/json"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/leaderelection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/system"
	"knative.dev/pkg/tracker"

	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
	pingsourceinformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1alpha2/pingsource"
	pingsourcereconciler "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1alpha2/pingsource"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount"
	"knative.dev/pkg/client/injection/kube/informers/rbac/v1/rolebinding"
)

// envConfig will be used to extract the required environment variables using
// github.com/kelseyhightower/envconfig. If this configuration cannot be extracted, then
// NewController will panic.
type envConfig struct {
	Image   string `envconfig:"PING_IMAGE" required:"true"`
	MTImage string `envconfig:"MT_PING_IMAGE" required:"true"`
}

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	deploymentInformer := deploymentinformer.Get(ctx)
	pingSourceInformer := pingsourceinformer.Get(ctx)
	serviceAccountInformer := serviceaccount.Get(ctx)
	roleBindingInformer := rolebinding.Get(ctx)

	// Retrieve leader election config
	leaderElectionConfig, err := sharedmain.GetLeaderElectionConfig(ctx)
	if err != nil {
		logger.Fatalw("Error loading leader election configuration", zap.Error(err))
	}

	leConfig, err := leComponentConfigToJson(leaderElectionConfig.GetComponentConfig(mtcomponent))
	if err != nil {
		logger.Fatalw("Error converting leader election configuration to JSON", zap.Error(err))
	}

	r := &Reconciler{
		kubeClientSet:        kubeclient.Get(ctx),
		pingLister:           pingSourceInformer.Lister(),
		deploymentLister:     deploymentInformer.Lister(),
		serviceAccountLister: serviceAccountInformer.Lister(),
		roleBindingLister:    roleBindingInformer.Lister(),
		leConfig:             leConfig,
		loggingContext:       ctx,
	}

	env := &envConfig{}
	if err := envconfig.Process("", env); err != nil {
		logging.FromContext(ctx).Panicf("unable to process PingSourceSource's required environment variables: %v", err)
	}
	r.receiveAdapterImage = env.Image
	r.receiveMTAdapterImage = env.MTImage

	impl := pingsourcereconciler.NewImpl(ctx, r)

	r.sinkResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	logger.Info("Setting up event handlers")
	pingSourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Watch for deployments owned by the source
	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(v1alpha2.Kind("PingSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	// Tracker is used to notify us that the jobrunner Deployment has changed so that
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

	cmw.Watch(logging.ConfigMapName(), r.UpdateFromLoggingConfigMap)
	cmw.Watch(metrics.ConfigMapName(), r.UpdateFromMetricsConfigMap)

	return impl
}

// leComponentConfigToJson converts a ComponentConfig to a json string.
// TODO: move to pkg
func leComponentConfigToJson(cfg leaderelection.ComponentConfig) (string, error) {
	jsonCfg, err := json.Marshal(cfg)
	return string(jsonCfg), err
}
