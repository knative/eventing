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

	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/filtered"
	"knative.dev/pkg/system"

	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/eventingtls"
	eventingreconciler "knative.dev/eventing/pkg/reconciler"

	"knative.dev/eventing/pkg/apis/feature"

	"github.com/kelseyhightower/envconfig"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"

	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/namespace"

	serviceaccountinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/filtered"

	roleinformer "knative.dev/pkg/client/injection/kube/informers/rbac/v1/role/filtered"
	rolebindinginformer "knative.dev/pkg/client/injection/kube/informers/rbac/v1/rolebinding/filtered"

	apiserversourceinformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1/apiserversource"
	apiserversourcereconciler "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1/apiserversource"
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
	namespaceInformer := namespace.Get(ctx)
	oidcServiceaccountInformer := serviceaccountinformer.Get(ctx, auth.OIDCLabelSelector)

	// Create a selector string
	roleInformer := roleinformer.Get(ctx, auth.OIDCLabelSelector)
	rolebindingInformer := rolebindinginformer.Get(ctx, auth.OIDCLabelSelector)

	trustBundleConfigMapInformer := configmapinformer.Get(ctx, eventingtls.TrustBundleLabelSelector)

	var globalResync func(obj interface{})

	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"), func(name string, value interface{}) {
		if globalResync != nil {
			globalResync(nil)
		}
	})
	featureStore.WatchConfigs(cmw)

	r := &Reconciler{
		kubeClientSet:              kubeclient.Get(ctx),
		ceSource:                   GetCfgHost(ctx),
		configs:                    reconcilersource.WatchConfigurations(ctx, component, cmw),
		namespaceLister:            namespaceInformer.Lister(),
		serviceAccountLister:       oidcServiceaccountInformer.Lister(),
		roleLister:                 roleInformer.Lister(),
		roleBindingLister:          rolebindingInformer.Lister(),
		trustBundleConfigMapLister: trustBundleConfigMapInformer.Lister(),
	}

	env := &envConfig{}
	if err := envconfig.Process("", env); err != nil {
		logging.FromContext(ctx).Panicf("unable to process APIServerSource's required environment variables: %v", err)
	}
	r.receiveAdapterImage = env.Image

	impl := apiserversourcereconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			ConfigStore: featureStore,
		}
	})

	globalResync = func(interface{}) {
		impl.GlobalResync(apiServerSourceInformer.Informer())
	}

	r.sinkResolver = resolver.NewURIResolverFromTracker(ctx, impl.Tracker)

	apiServerSourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&v1.ApiServerSource{}),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	roleInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&v1.ApiServerSource{}),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	rolebindingInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&v1.ApiServerSource{}),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	cb := func() {
		logging.FromContext(ctx).Info("Global resync of APIServerSources due to namespaces changing.")
		impl.GlobalResync(apiServerSourceInformer.Informer())
	}

	namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { cb() },
		UpdateFunc: func(oldObj, newObj interface{}) { cb() },
		DeleteFunc: func(obj interface{}) { cb() },
	})

	// Reconciler ApiServerSource when the OIDC service account changes
	oidcServiceaccountInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&v1.ApiServerSource{}),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	trustBundleConfigMapInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: eventingreconciler.FilterWithNamespace(system.Namespace()),
		Handler:    controller.HandleAll(globalResync),
	})

	return impl
}
