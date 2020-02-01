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

package apiserversource

import (
	"context"
	"knative.dev/eventing/pkg/client/injection/reconciler/sources/v1alpha1/apiserversource"
	"knative.dev/eventing/pkg/reconciler"
	kubeclient "knative.dev/pkg/client/injection/kube/client"

	"github.com/kelseyhightower/envconfig"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/resolver"

	eventtypeinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventtype"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"

	"knative.dev/pkg/logging"
)

const (
	controllerAgentName = "apiserversource-controller"
	finalizerName       = "apiserversource"
)

// envConfig will be used to extract the required environment variables using
// github.com/kelseyhightower/envconfig. If this configuration cannot be extracted, then
// NewController will panic.
type envConfig struct {
	Image string `envconfig:"APISERVER_RA_IMAGE" required:"true"`
}

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	deploymentInformer := deploymentinformer.Get(ctx)
	eventTypeInformer := eventtypeinformer.Get(ctx)

	r := &Reconciler{
		KubeClientSet:    kubeclient.Get(ctx),
		deploymentLister: deploymentInformer.Lister(),
		eventTypeLister:  eventTypeInformer.Lister(),
		source:           GetCfgHost(ctx),
		loggingContext:   ctx,
	}
	impl := apiserversource.NewImpl(ctx, r)

	// TODO: push this into the generated client.
	statsReporter := reconciler.GetStatsReporter(ctx)
	if statsReporter == nil {
		logging.FromContext(ctx).Debug("Creating stats reporter")
		var err error
		statsReporter, err = reconciler.NewStatsReporter(controllerAgentName)
		if err != nil {
			logging.FromContext(ctx).Fatal(err)
		}
	}
	r.StatsReporter = statsReporter

	env := &envConfig{}
	if err := envconfig.Process("", env); err != nil {
		logging.FromContext(ctx).Panicf("unable to process APIServerSource's required environment variables: %v", err)
	}
	r.receiveAdapterImage = env.Image
	r.sinkResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	logging.FromContext(ctx).Info("Setting up additional handlers")

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("ApiServerSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	eventTypeInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("ApiServerSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	cmw.Watch(logging.ConfigMapName(), r.UpdateFromLoggingConfigMap)
	cmw.Watch(metrics.ConfigMapName(), r.UpdateFromMetricsConfigMap)

	return impl
}
