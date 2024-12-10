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

package sink

import (
	"context"
	pkgreconciler "knative.dev/pkg/reconciler"

	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/feature"
	v1alpha1 "knative.dev/eventing/pkg/apis/sinks/v1alpha1"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventpolicy"
	"knative.dev/eventing/pkg/client/injection/informers/sinks/v1alpha1/integrationsink"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/service"

	cmclient "knative.dev/eventing/pkg/client/certmanager/injection/client"
	cmcertinformer "knative.dev/eventing/pkg/client/certmanager/injection/informers/certmanager/v1/certificate"

	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"

	integrationsinkreconciler "knative.dev/eventing/pkg/client/injection/reconciler/sinks/v1alpha1/integrationsink"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
)

// NewController creates a Reconciler for IntegrationSource and returns the result of NewImpl.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	integrationSinkInformer := integrationsink.Get(ctx)
	secretInformer := secretinformer.Get(ctx)
	eventPolicyInformer := eventpolicy.Get(ctx)
	deploymentInformer := deploymentinformer.Get(ctx)

	cmCertificateInformer := cmcertinformer.Get(ctx)

	serviceInformer := service.Get(ctx)

	r := &Reconciler{
		kubeClientSet: kubeclient.Get(ctx),

		deploymentLister: deploymentInformer.Lister(),
		serviceLister:    serviceInformer.Lister(),

		systemNamespace:     system.Namespace(),
		secretLister:        secretInformer.Lister(),
		eventPolicyLister:   eventPolicyInformer.Lister(),
		cmCertificateLister: cmCertificateInformer.Lister(),
		certManagerClient:   cmclient.Get(ctx),
	}

	var globalResync func(obj interface{})

	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"), func(name string, value interface{}) {
		if globalResync != nil {
			globalResync(nil)
		}
	})
	featureStore.WatchConfigs(cmw)

	impl := integrationsinkreconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			ConfigStore: featureStore,
		}
	})

	integrationSinkInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	globalResync = func(interface{}) {
		impl.GlobalResync(integrationSinkInformer.Informer())
	}

	secretInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: pkgreconciler.LabelFilterFunc("app.kubernetes.io/name", "integration-sink", false),
		Handler:    controller.HandleAll(globalResync),
	})

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGVK(v1alpha1.SchemeGroupVersion.WithKind("IntegrationSink")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	integrationSinkGK := v1alpha1.SchemeGroupVersion.WithKind("IntegrationSink").GroupKind()

	// Enqueue the JobSink, if we have an EventPolicy which was referencing
	// or got updated and now is referencing the JobSink.
	eventPolicyInformer.Informer().AddEventHandler(auth.EventPolicyEventHandler(
		integrationSinkInformer.Informer().GetIndexer(),
		integrationSinkGK,
		impl.EnqueueKey,
	))

	return impl
}
