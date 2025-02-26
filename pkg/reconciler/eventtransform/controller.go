/*
Copyright 2025 The Knative Authors

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

package eventtransform

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment/filtered"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/filtered"
	serviceinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/service/filtered"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/apis/feature"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	eventtransformeryinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventtransform"
	sinkbindinginformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1/sinkbinding/filtered"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1alpha1/eventtransform"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
)

const (
	NameLabelKey = "eventing.knative.dev/event-transform-name"
)

func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	eventTransformInformer := eventtransformeryinformer.Get(ctx)
	jsonataConfigMapInformer := configmapinformer.Get(ctx, JsonataResourcesSelector)
	jsonataDeploymentInformer := deploymentinformer.Get(ctx, JsonataResourcesSelector)
	jsonataSinkBindingInformer := sinkbindinginformer.Get(ctx, JsonataResourcesSelector)
	jsonataServiceInformer := serviceinformer.Get(ctx, JsonataResourcesSelector)

	// Create a custom informer as one in knative/pkg doesn't exist for endpoints.
	factory := informers.NewSharedInformerFactoryWithOptions(
		kubeclient.Get(ctx),
		controller.DefaultResyncPeriod,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = JsonataResourcesSelector
		}),
	)
	jsonataEndpointInformer := factory.Core().V1().Endpoints()

	var globalResync func()

	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"), func(name string, value interface{}) {
		if globalResync != nil {
			globalResync()
		}
	})
	featureStore.WatchConfigs(cmw)

	configWatcher := reconcilersource.WatchConfigurations(ctx, "eventtransform", cmw,
		reconcilersource.WithTracing,
	)

	r := &Reconciler{
		k8s:                      kubeclient.Get(ctx),
		client:                   eventingclient.Get(ctx),
		jsonataConfigMapLister:   jsonataConfigMapInformer.Lister(),
		jsonataDeploymentsLister: jsonataDeploymentInformer.Lister(),
		jsonataServiceLister:     jsonataServiceInformer.Lister(),
		jsonataEndpointLister:    jsonataEndpointInformer.Lister(),
		jsonataSinkBindingLister: jsonataSinkBindingInformer.Lister(),
		configWatcher:            configWatcher,
	}

	impl := eventtransform.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			ConfigStore: featureStore,
		}
	})

	globalResync = func() {
		impl.GlobalResync(eventTransformInformer.Informer())
	}

	eventTransformInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	jsonataDeploymentInformer.Informer().AddEventHandler(controller.HandleAll(enqueueUsingNameLabel(impl)))
	jsonataServiceInformer.Informer().AddEventHandler(controller.HandleAll(enqueueUsingNameLabel(impl)))
	jsonataEndpointInformer.Informer().AddEventHandler(controller.HandleAll(enqueueUsingNameLabel(impl)))
	jsonataConfigMapInformer.Informer().AddEventHandler(controller.HandleAll(enqueueUsingNameLabel(impl)))
	jsonataSinkBindingInformer.Informer().AddEventHandler(controller.HandleAll(enqueueUsingNameLabel(impl)))

	// Start the factory after creating all necessary informers.
	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())

	return impl
}

func enqueueUsingNameLabel(impl *controller.Impl) func(obj interface{}) {
	return func(obj interface{}) {
		acc, err := kmeta.DeletionHandlingAccessor(obj)
		if err != nil {
			return
		}
		name, ok := acc.GetLabels()[NameLabelKey]
		if !ok {
			return
		}
		impl.EnqueueKey(types.NamespacedName{Namespace: acc.GetNamespace(), Name: name})
	}
}
