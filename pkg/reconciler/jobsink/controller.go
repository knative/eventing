/*
Copyright 2024 The Knative Authors

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

package jobsink

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"

	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/system"

	jobinformer "knative.dev/pkg/client/injection/kube/informers/batch/v1/job/filtered"
	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"

	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventing/pkg/apis/sinks"
	"knative.dev/eventing/pkg/client/injection/informers/sinks/v1alpha1/jobsink"
	jobsinkreconciler "knative.dev/eventing/pkg/client/injection/reconciler/sinks/v1alpha1/jobsink"
	"knative.dev/eventing/pkg/eventingtls"
)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	jobSinkInformer := jobsink.Get(ctx)
	secretInformer := secretinformer.Get(ctx)
	jobInformer := jobinformer.Get(ctx, sinks.JobSinkJobsLabelSelector)

	r := &Reconciler{
		systemNamespace: system.Namespace(),
		secretLister:    secretInformer.Lister(),
		jobLister:       jobInformer.Lister(),
	}

	var globalResync func(obj interface{})

	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"), func(name string, value interface{}) {
		if globalResync != nil {
			globalResync(nil)
		}
	})
	featureStore.WatchConfigs(cmw)

	impl := jobsinkreconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			ConfigStore: featureStore,
		}
	})

	jobSinkInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	globalResync = func(interface{}) {
		impl.GlobalResync(jobSinkInformer.Informer())
	}
	secretInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithName(eventingtls.JobSinkDispatcherServerTLSSecretName),
		Handler:    controller.HandleAll(globalResync),
	})

	jobInformer.Informer().AddEventHandler(controller.HandleAll(func(i interface{}) {
		obj, err := kmeta.DeletionHandlingAccessor(i)
		if err != nil {
			return
		}
		name, ok := obj.GetLabels()[sinks.JobSinkNameLabel]
		if !ok {
			return
		}
		impl.EnqueueKey(types.NamespacedName{
			Namespace: obj.GetNamespace(),
			Name:      name,
		})
	}))

	return impl
}
