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

package namespace

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	sugarconfig "knative.dev/eventing/pkg/apis/sugar"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/pkg/client/injection/informers/eventing/v1/broker"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/namespace"
	namespacereconciler "knative.dev/pkg/client/injection/kube/reconciler/core/v1/namespace"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	namespaceInformer := namespace.Get(ctx)
	brokerInformer := broker.Get(ctx)

	r := &Reconciler{
		eventingClientSet: eventingclient.Get(ctx),
		brokerLister:      brokerInformer.Lister(),
	}

	impl := namespacereconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {

		configStore := sugarconfig.NewStore(logging.FromContext(ctx).Named("config-sugar-store"))
		configStore.WatchConfigs(cmw)

		return controller.Options{
			SkipStatusUpdates: true,
			ConfigStore:       configStore,
		}
	})

	namespaceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))
	brokerInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterControllerGK(corev1.SchemeGroupVersion.WithKind("Namespace").GroupKind()),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

	// When brokers change, change perform a global resync on namespaces.
	grCb := func(obj interface{}) {
		logging.FromContext(ctx).Info("Doing a global resync on Namespaces due to Brokers changing.")
		impl.GlobalResync(namespaceInformer.Informer())
	}
	// Resync on deleting of brokers.
	brokerInformer.Informer().AddEventHandler(HandleOnlyDelete(grCb))

	return impl
}

func HandleOnlyDelete(h func(interface{})) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) {},
		UpdateFunc: func(oldObj, newObj interface{}) {},
		DeleteFunc: h,
	}
}
