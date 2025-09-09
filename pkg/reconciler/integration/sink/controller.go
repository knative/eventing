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

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/eventingtls"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/filtered"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/system"

	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/injection"

	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventing/pkg/apis/sinks/v1alpha1"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/certificates"
	"knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventpolicy"
	"knative.dev/eventing/pkg/client/injection/informers/sinks/v1alpha1/integrationsink"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/service"

	pkgreconciler "knative.dev/pkg/reconciler"

	certmanagerclientset "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"

	integrationsinkreconciler "knative.dev/eventing/pkg/client/injection/reconciler/sinks/v1alpha1/integrationsink"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	secretinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/secret/filtered"
	rolebindinginformer "knative.dev/pkg/client/injection/kube/informers/rbac/v1/rolebinding"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// envConfig will be used to extract the required environment variables.
// If this configuration cannot be extracted, then NewController will panic.
type envConfig struct {
	AuthProxyImage string `envconfig:"AUTH_PROXY_IMAGE" required:"true"`
}

func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	integrationSinkInformer := integrationsink.Get(ctx)
	secretInformer := secretinformer.Get(ctx, certificates.SecretLabelSelectorPair)
	eventPolicyInformer := eventpolicy.Get(ctx)
	deploymentInformer := deploymentinformer.Get(ctx)
	serviceInformer := service.Get(ctx)
	trustBundleConfigMapInformer := configmapinformer.Get(ctx, eventingtls.TrustBundleLabelSelector)
	rolebindingInformer := rolebindinginformer.Get(ctx)
	dynamicCertificateInformer := certificates.NewDynamicCertificatesInformer()

	env := &envConfig{}
	if err := envconfig.Process("", env); err != nil {
		logging.FromContext(ctx).Panicf("unable to process IntegrationSink's required environment variables: %v", err)
	}

	r := &Reconciler{
		secretLister:               secretInformer.Lister(),
		eventPolicyLister:          eventPolicyInformer.Lister(),
		kubeClientSet:              kubeclient.Get(ctx),
		deploymentLister:           deploymentInformer.Lister(),
		serviceLister:              serviceInformer.Lister(),
		cmCertificateLister:        dynamicCertificateInformer.Lister(),
		certManagerClient:          certmanagerclientset.NewForConfigOrDie(injection.GetConfig(ctx)),
		trustBundleConfigMapLister: trustBundleConfigMapInformer.Lister(),
		integrationSinkLister:      integrationSinkInformer.Lister(),
		rolebindingLister:          rolebindingInformer.Lister(),
		authProxyImage:             env.AuthProxyImage,
	}

	logging.FromContext(ctx).Info("Creating IntegrationSink controller")

	var globalResync func(obj interface{})
	var enqueueControllerOf func(interface{})

	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"), func(name string, value interface{}) {
		if globalResync != nil {
			globalResync(nil)
		}

		if features, ok := value.(feature.Flags); ok && enqueueControllerOf != nil {
			// we assume that Cert-Manager is installed in the cluster if the feature flag is enabled
			if err := dynamicCertificateInformer.Reconcile(ctx, features, controller.HandleAll(enqueueControllerOf)); err != nil {
				logging.FromContext(ctx).Errorw("Failed to start certificates dynamic factory", zap.Error(err))
			}
		}
	})
	featureStore.WatchConfigs(cmw)

	impl := integrationsinkreconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			ConfigStore: featureStore,
		}
	})
	enqueueControllerOf = impl.EnqueueControllerOf

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

	serviceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGVK(v1alpha1.SchemeGroupVersion.WithKind("IntegrationSink")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	integrationSinkGK := v1alpha1.SchemeGroupVersion.WithKind("IntegrationSink").GroupKind()
	eventPolicyInformer.Informer().AddEventHandler(auth.EventPolicyEventHandler(
		integrationSinkInformer.Informer().GetIndexer(),
		integrationSinkGK,
		impl.EnqueueKey,
	))

	trustBundleConfigMapInformer.Informer().AddEventHandler(controller.HandleAll(func(i interface{}) {
		obj, err := kmeta.DeletionHandlingAccessor(i)
		if err != nil {
			return
		}
		if obj.GetNamespace() == system.Namespace() {
			globalResync(i)
			return
		}

		sinks, err := integrationSinkInformer.Lister().IntegrationSinks(obj.GetNamespace()).List(labels.Everything())
		if err != nil {
			return
		}
		for _, sink := range sinks {
			impl.EnqueueKey(types.NamespacedName{
				Namespace: sink.Namespace,
				Name:      sink.Name,
			})
		}
	}))

	return impl
}
