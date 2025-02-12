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

	"k8s.io/apimachinery/pkg/types"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment/filtered"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/filtered"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/apis/feature"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	eventtransformeryinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventtransform"
	sinkbindinginformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1/sinkbinding/filtered"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1alpha1/eventtransform"
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

	var globalResync func()

	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"), func(name string, value interface{}) {
		if globalResync != nil {
			globalResync()
		}
	})
	featureStore.WatchConfigs(cmw)

	r := &Reconciler{
		k8s:                      kubeclient.Get(ctx),
		client:                   eventingclient.Get(ctx),
		jsonataConfigMapLister:   jsonataConfigMapInformer.Lister(),
		jsonataDeploymentsLister: jsonataDeploymentInformer.Lister(),
		jsonataSinkBindingLister: jsonataSinkBindingInformer.Lister(),
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
	jsonataConfigMapInformer.Informer().AddEventHandler(controller.HandleAll(enqueueUsingNameLabel(impl)))
	jsonataSinkBindingInformer.Informer().AddEventHandler(controller.HandleAll(enqueueUsingNameLabel(impl)))

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
