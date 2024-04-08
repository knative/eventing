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

package sinkbinding

import (
	"context"
	"time"

	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/system"

	"knative.dev/eventing/pkg/auth"
	sbinformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1/sinkbinding"
	"knative.dev/eventing/pkg/eventingtls"

	"knative.dev/pkg/client/injection/ducks/duck/v1/podspecable"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/namespace"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis/duck"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/filtered"
	secretinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/secret"
	serviceaccountinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/filtered"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracker"
	"knative.dev/pkg/webhook/psbinding"

	"knative.dev/eventing/pkg/apis/feature"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
)

const (
	controllerAgentName = "sinkbinding-controller"

	// resyncPeriod defines the period in which SinkBindings will be reenqued
	// (e.g. to check the validity of their OIDC token secret)
	resyncPeriod = auth.TokenExpirationTime / 2
	// tokenExpiryBuffer defines an additional buffer for the expiry of OIDC
	// token secrets
	tokenExpiryBuffer = 5 * time.Minute
)

// NewController returns a new SinkBinding reconciler.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	sbInformer := sbinformer.Get(ctx)
	dc := dynamicclient.Get(ctx)
	psInformerFactory := podspecable.Get(ctx)
	namespaceInformer := namespace.Get(ctx)
	oidcServiceaccountInformer := serviceaccountinformer.Get(ctx, auth.OIDCLabelSelector)
	secretInformer := secretinformer.Get(ctx)
	trustBundleConfigMapInformer := configmapinformer.Get(ctx, eventingtls.TrustBundleLabelSelector)
	trustBundleConfigMapLister := trustBundleConfigMapInformer.Lister()

	var globalResync func()
	featureStore := feature.NewStore(logging.FromContext(ctx).Named("feature-config-store"), func(name string, value interface{}) {
		logger.Infof("feature config changed. name: %s, value: %v", name, value)

		if globalResync != nil {
			globalResync()
		}
	})
	featureStore.WatchConfigs(cmw)

	c := &psbinding.BaseReconciler{
		LeaderAwareFuncs: reconciler.LeaderAwareFuncs{
			PromoteFunc: func(bkt reconciler.Bucket, enq func(reconciler.Bucket, types.NamespacedName)) error {
				all, err := sbInformer.Lister().List(labels.Everything())
				if err != nil {
					return err
				}
				for _, elt := range all {
					enq(bkt, types.NamespacedName{
						Namespace: elt.GetNamespace(),
						Name:      elt.GetName(),
					})
				}
				return nil
			},
		},
		GVR: v1.SchemeGroupVersion.WithResource("sinkbindings"),
		Get: func(namespace string, name string) (psbinding.Bindable, error) {
			return sbInformer.Lister().SinkBindings(namespace).Get(name)
		},
		DynamicClient:   dc,
		Recorder:        createRecorder(ctx, controllerAgentName),
		NamespaceLister: namespaceInformer.Lister(),
	}
	impl := controller.NewContext(ctx, c, controller.ControllerOptions{
		WorkQueueName: "SinkBindings",
		Logger:        logger,
	})

	globalResync = func() {
		impl.GlobalResync(sbInformer.Informer())
	}

	sbInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))
	namespaceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	sbResolver := resolver.NewURIResolverFromTracker(ctx, impl.Tracker)
	c.SubResourcesReconciler = &SinkBindingSubResourcesReconciler{
		res:                        sbResolver,
		tracker:                    impl.Tracker,
		kubeclient:                 kubeclient.Get(ctx),
		serviceAccountLister:       oidcServiceaccountInformer.Lister(),
		secretLister:               secretInformer.Lister(),
		featureStore:               featureStore,
		tokenProvider:              auth.NewOIDCTokenProvider(ctx),
		trustBundleConfigMapLister: trustBundleConfigMapLister,
	}

	c.WithContext = func(ctx context.Context, b psbinding.Bindable) (context.Context, error) {
		return v1.WithTrustBundleConfigMapLister(v1.WithURIResolver(ctx, sbResolver), trustBundleConfigMapLister), nil
	}
	c.Tracker = impl.Tracker
	c.Factory = &duck.CachedInformerFactory{
		Delegate: &duck.EnqueueInformerFactory{
			Delegate:     psInformerFactory,
			EventHandler: controller.HandleAll(c.Tracker.OnChanged),
		},
	}

	// Reconcile SinkBinding when the OIDC service account changes
	oidcServiceaccountInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&v1.SinkBinding{}),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	// do a periodic reync of all sinkbindings to renew the token secrets eventually
	go periodicResync(ctx, globalResync)

	trustBundleConfigMapInformer.Informer().AddEventHandler(controller.HandleAll(func(i interface{}) {
		obj, err := kmeta.DeletionHandlingAccessor(i)
		if err != nil {
			return
		}
		if obj.GetNamespace() == system.Namespace() {
			globalResync()
			return
		}

		sbs, err := sbInformer.Lister().SinkBindings(obj.GetNamespace()).List(labels.Everything())
		if err != nil {
			return
		}
		for _, sb := range sbs {
			impl.EnqueueKey(types.NamespacedName{
				Namespace: sb.Namespace,
				Name:      sb.Name,
			})
		}
	}))

	return impl
}

func periodicResync(ctx context.Context, globalResyncFunc func()) {
	ticker := time.NewTicker(resyncPeriod)
	logger := logging.FromContext(ctx)

	logger.Infof("Starting global resync of SinkBindings every %s", resyncPeriod)
	for {
		select {
		case <-ticker.C:
			logger.Debug("Triggering global resync of SinkBindings")
			globalResyncFunc()
		case <-ctx.Done():
			logger.Debug("Context finished. Stopping periodic resync of SinkBindings")
			return
		}
	}
}

func ListAll(ctx context.Context, handler cache.ResourceEventHandler) psbinding.ListAll {
	fbInformer := sbinformer.Get(ctx)

	// Whenever a SinkBinding changes our webhook programming might change.
	fbInformer.Informer().AddEventHandler(handler)

	return func() ([]psbinding.Bindable, error) {
		l, err := fbInformer.Lister().List(labels.Everything())
		if err != nil {
			return nil, err
		}
		bl := make([]psbinding.Bindable, 0, len(l))
		for _, elt := range l {
			bl = append(bl, elt)
		}
		return bl, nil
	}

}

func WithContextFactory(ctx context.Context, lister corev1listers.ConfigMapLister, handler func(types.NamespacedName)) psbinding.BindableContext {
	r := resolver.NewURIResolverFromTracker(ctx, tracker.New(handler, controller.GetTrackerLease(ctx)))

	return func(ctx context.Context, b psbinding.Bindable) (context.Context, error) {
		return v1.WithTrustBundleConfigMapLister(v1.WithURIResolver(ctx, r), lister), nil
	}
}

func createRecorder(ctx context.Context, agentName string) record.EventRecorder {
	logger := logging.FromContext(ctx)

	recorder := controller.GetEventRecorder(ctx)
	if recorder == nil {
		// Create event broadcaster
		logger.Debug("Creating event broadcaster")
		eventBroadcaster := record.NewBroadcaster()
		watches := []watch.Interface{
			eventBroadcaster.StartLogging(logger.Named("event-broadcaster").Infof),
			eventBroadcaster.StartRecordingToSink(
				&typedcorev1.EventSinkImpl{Interface: kubeclient.Get(ctx).CoreV1().Events("")}),
		}
		recorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: agentName})
		go func() {
			<-ctx.Done()
			for _, w := range watches {
				w.Stop()
			}
		}()
	}

	return recorder
}
