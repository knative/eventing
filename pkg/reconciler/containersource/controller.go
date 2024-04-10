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

package containersource

import (
	"context"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/filtered"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/system"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	serviceaccountinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/filtered"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/apis/feature"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/pkg/auth"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	containersourceinformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1/containersource"
	sinkbindinginformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1/sinkbinding"
	v1containersource "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1/containersource"
	"knative.dev/eventing/pkg/eventingtls"
)

// NewController creates a Reconciler for ContainerSource and returns the result of NewImpl.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	kubeClient := kubeclient.Get(ctx)
	eventingClient := eventingclient.Get(ctx)
	containersourceInformer := containersourceinformer.Get(ctx)
	sinkbindingInformer := sinkbindinginformer.Get(ctx)
	deploymentInformer := deploymentinformer.Get(ctx)
	oidcServiceaccountInformer := serviceaccountinformer.Get(ctx, auth.OIDCLabelSelector)
	trustBundleConfigMapInformer := configmapinformer.Get(ctx, eventingtls.TrustBundleLabelSelector)

	var globalResync func(obj interface{})
	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"),
		func(name string, value interface{}) {
			if globalResync != nil {
				globalResync(nil)
			}
		})
	featureStore.WatchConfigs(cmw)

	r := &Reconciler{
		kubeClientSet:              kubeClient,
		eventingClientSet:          eventingClient,
		containerSourceLister:      containersourceInformer.Lister(),
		deploymentLister:           deploymentInformer.Lister(),
		sinkBindingLister:          sinkbindingInformer.Lister(),
		serviceAccountLister:       oidcServiceaccountInformer.Lister(),
		trustBundleConfigMapLister: trustBundleConfigMapInformer.Lister(),
	}
	impl := v1containersource.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		return controller.Options{ConfigStore: featureStore}
	})

	globalResync = func(_ interface{}) {
		impl.GlobalResync(containersourceInformer.Informer())
	}

	containersourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGVK(v1.SchemeGroupVersion.WithKind("ContainerSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	sinkbindingInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&v1.ContainerSource{}),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	trustBundleConfigMapInformer.Informer().AddEventHandler(controller.HandleAll(func(i interface{}) {
		obj, err := kmeta.DeletionHandlingAccessor(i)
		if err != nil {
			return
		}
		if obj.GetNamespace() == system.Namespace() {
			globalResync(i)
			return
		}

		sources, err := containersourceInformer.Lister().ContainerSources(obj.GetNamespace()).List(labels.Everything())
		if err != nil {
			return
		}
		for _, src := range sources {
			impl.EnqueueKey(types.NamespacedName{
				Namespace: src.Namespace,
				Name:      src.Name,
			})
		}
	}))

	return impl
}
