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

	"github.com/kelseyhightower/envconfig"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"

	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"

	apiserversourceinformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1alpha2/apiserversource"
	apiserversourcereconciler "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1alpha2/apiserversource"
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
	apiServerSourceInformer := apiserversourceinformer.Get(ctx)

	r := &Reconciler{
		kubeClientSet:         kubeclient.Get(ctx),
		apiserversourceLister: apiServerSourceInformer.Lister(),
		ceSource:              GetCfgHost(ctx),
		loggingContext:        ctx,
		configs:               reconcilersource.StartWatchingSourceConfigurations(ctx, component, cmw),
	}

	env := &envConfig{}
	if err := envconfig.Process("", env); err != nil {
		logging.FromContext(ctx).Panicf("unable to process APIServerSource's required environment variables: %v", err)
	}
	r.receiveAdapterImage = env.Image

	impl := apiserversourcereconciler.NewImpl(ctx, r)

	r.sinkResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	logging.FromContext(ctx).Info("Setting up event handlers")
	apiServerSourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterGroupKind(v1alpha1.Kind("ApiServerSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
