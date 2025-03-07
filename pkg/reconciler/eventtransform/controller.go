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
	"knative.dev/eventing/pkg/certificates"
	cmclient "knative.dev/eventing/pkg/client/certmanager/clientset/versioned"
	"knative.dev/eventing/pkg/eventingtls"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment/filtered"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/filtered"
	secretinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/secret/filtered"
	serviceinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/service/filtered"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/apis/feature"
	certificatesinformer "knative.dev/eventing/pkg/client/certmanager/injection/informers/certmanager/v1/certificate"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	eventtransforminformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventtransform"
	sinkbindinginformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1/sinkbinding/filtered"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1alpha1/eventtransform"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
)

const (
	NameLabelKey = "eventing.knative.dev/event-transform-name"
)

func init() {
	// TODO: Use dynamic (filtered) informer factory since cert-manager is an optional dependency: https://github.com/knative/eventing/pull/8517
	injection.Default.RegisterInformer(certificatesinformer.WithInformer)
}

func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	eventTransformInformer := eventtransforminformer.Get(ctx)
	jsonataConfigMapInformer := configmapinformer.Get(ctx, JsonataResourcesSelector)
	jsonataDeploymentInformer := deploymentinformer.Get(ctx, JsonataResourcesSelector)
	jsonataSinkBindingInformer := sinkbindinginformer.Get(ctx, JsonataResourcesSelector)
	jsonataServiceInformer := serviceinformer.Get(ctx, JsonataResourcesSelector)
	certificatesSecretInformer := secretinformer.Get(ctx, certificates.SecretLabelSelectorPair)
	trustBundleConfigMapInformer := configmapinformer.Get(ctx, eventingtls.TrustBundleLabelSelector)
	// TODO: Use dynamic (filtered) informer factory since cert-manager is an optional dependency: https://github.com/knative/eventing/pull/8517
	// 		Also remove init function
	certificatesInformer := certificatesinformer.Get(ctx)

	// Create a custom informer as one in knative/pkg doesn't exist for endpoints.
	jsonataEndpointFactory := informers.NewSharedInformerFactoryWithOptions(
		kubeclient.Get(ctx),
		controller.DefaultResyncPeriod,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = JsonataResourcesSelector
		}),
	)
	jsonataEndpointInformer := jsonataEndpointFactory.Core().V1().Endpoints()

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
		k8s:                        kubeclient.Get(ctx),
		client:                     eventingclient.Get(ctx),
		cmClient:                   cmclient.NewForConfigOrDie(injection.GetConfig(ctx)),
		jsonataConfigMapLister:     jsonataConfigMapInformer.Lister(),
		jsonataDeploymentsLister:   jsonataDeploymentInformer.Lister(),
		jsonataServiceLister:       jsonataServiceInformer.Lister(),
		jsonataEndpointLister:      jsonataEndpointInformer.Lister(),
		jsonataSinkBindingLister:   jsonataSinkBindingInformer.Lister(),
		cmCertificateLister:        certificatesInformer.Lister(),
		certificatesSecretLister:   certificatesSecretInformer.Lister(),
		trustBundleConfigMapLister: trustBundleConfigMapInformer.Lister(),
		configWatcher:              configWatcher,
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
	certificatesInformer.Informer().AddEventHandler(controller.HandleAll(impl.EnqueueControllerOf))

	// Start the factory after creating all necessary informers.
	jsonataEndpointFactory.Start(ctx.Done())
	jsonataEndpointFactory.WaitForCacheSync(ctx.Done())

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
