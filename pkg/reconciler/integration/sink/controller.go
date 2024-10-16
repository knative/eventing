package sink

import (
	"context"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventpolicy"
	"knative.dev/eventing/pkg/client/injection/informers/sinks/v1alpha1/integrationsink"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/service"

	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"

	integrationsinkreconciler "knative.dev/eventing/pkg/client/injection/reconciler/sinks/v1alpha1/integrationsink"
	"knative.dev/eventing/pkg/eventingtls"
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

	serviceInformer := service.Get(ctx)

	r := &Reconciler{
		kubeClientSet: kubeclient.Get(ctx),

		deploymentLister: deploymentInformer.Lister(),
		serviceLister:    serviceInformer.Lister(),

		systemNamespace:   system.Namespace(),
		secretLister:      secretInformer.Lister(),
		eventPolicyLister: eventPolicyInformer.Lister(),
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
		FilterFunc: controller.FilterWithName(eventingtls.IntegrationSinkDispatcherServerTLSSecretName),
		Handler:    controller.HandleAll(globalResync),
	})

	//integrationSinkGK := sinksv1alpha1.SchemeGroupVersion.WithKind("IntegrationSink").GroupKind()
	//
	//// Enqueue the JobSink, if we have an EventPolicy which was referencing
	//// or got updated and now is referencing the JobSink.
	//eventPolicyInformer.Informer().AddEventHandler(auth.EventPolicyEventHandler(
	//	integrationSinkInformer.Informer().GetIndexer(),
	//	integrationSinkGK,
	//	impl.EnqueueKey,
	//))
	//
	return impl
}
