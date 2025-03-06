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

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	certmanagerclient "knative.dev/eventing/pkg/client/certmanager/clientset/versioned"
	certmanagerinformers "knative.dev/eventing/pkg/client/certmanager/informers/externalversions"
	"knative.dev/pkg/injection"

	"knative.dev/eventing/pkg/apis/feature"
	v1alpha1 "knative.dev/eventing/pkg/apis/sinks/v1alpha1"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventpolicy"
	"knative.dev/eventing/pkg/client/injection/informers/sinks/v1alpha1/integrationsink"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/service"
	pkgreconciler "knative.dev/pkg/reconciler"

	integrationsinkreconciler "knative.dev/eventing/pkg/client/injection/reconciler/sinks/v1alpha1/integrationsink"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	secretinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/secret/filtered"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	integrationSinkInformer := integrationsink.Get(ctx)
	secretInformer := secretinformer.Get(ctx, "app.kubernetes.io/name")
	eventPolicyInformer := eventpolicy.Get(ctx)
	deploymentInformer := deploymentinformer.Get(ctx)
	serviceInformer := service.Get(ctx)

	r := &Reconciler{
		kubeClientSet:     kubeclient.Get(ctx),
		deploymentLister:  deploymentInformer.Lister(),
		serviceLister:     serviceInformer.Lister(),
		secretLister:      secretInformer.Lister(),
		eventPolicyLister: eventPolicyInformer.Lister(),
	}

	logging.FromContext(ctx).Info("Creating IntegrationSink controller")

	var globalResync func(obj interface{})

	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"), func(name string, value interface{}) {
		if globalResync != nil {
			globalResync(nil)
		}

		if features, ok := value.(feature.Flags); ok {
			// we assume that Cert-Manager is installed in the cluster if the feature flag is enabled
			if features.IsPermissiveTransportEncryption() || features.IsStrictTransportEncryption() {

				go func() { // Start and register informer
					certManagerClient := certmanagerclient.NewForConfigOrDie(injection.GetConfig(ctx))
					factory := certmanagerinformers.NewSharedInformerFactoryWithOptions(
						certManagerClient,
						controller.DefaultResyncPeriod,
						certmanagerinformers.WithTweakListOptions(func(options *v1.ListOptions) {
							options.LabelSelector = "app.kubernetes.io/component=knative-eventing"
						}),
					)
					cmCertificateInformer := factory.Certmanager().V1().Certificates()
					r.cmCertificateLister = cmCertificateInformer.Lister()
					r.certManagerClient = certManagerClient

					cmCertificateInformer.Informer().AddEventHandler(controller.HandleAll(globalResync))
					factory.Start(ctx.Done())

					if !cache.WaitForCacheSync(ctx.Done(), cmCertificateInformer.Informer().HasSynced) {
						logging.FromContext(ctx).Error("Failed to sync Cert Manager Certificate informer")
					}
				}()

			} else {
				cancelFunc, cancelExists := ctx.Value("cancelFunc").(context.CancelFunc)
				if cancelExists {
					go cancelFunc()
				}
			}
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
	eventPolicyInformer.Informer().AddEventHandler(auth.EventPolicyEventHandler(
		integrationSinkInformer.Informer().GetIndexer(),
		integrationSinkGK,
		impl.EnqueueKey,
	))

	return impl
}
