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
	"errors"

	sbinformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1/sinkbinding"
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
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/pkg/apis/duck"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracker"
	"knative.dev/pkg/webhook/psbinding"
)

const (
	controllerAgentName = "sinkbinding-controller"
)

type SinkBindingSubResourcesReconciler struct {
	res     *resolver.URIResolver
	tracker tracker.Interface
}

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

	sbInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))
	namespaceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	sbResolver := resolver.NewURIResolverFromTracker(ctx, impl.Tracker)
	c.SubResourcesReconciler = &SinkBindingSubResourcesReconciler{
		res:     sbResolver,
		tracker: impl.Tracker,
	}

	c.WithContext = func(ctx context.Context, b psbinding.Bindable) (context.Context, error) {
		return v1.WithURIResolver(ctx, sbResolver), nil
	}
	c.Tracker = impl.Tracker
	c.Factory = &duck.CachedInformerFactory{
		Delegate: &duck.EnqueueInformerFactory{
			Delegate:     psInformerFactory,
			EventHandler: controller.HandleAll(c.Tracker.OnChanged),
		},
	}

	return impl
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

func WithContextFactory(ctx context.Context, handler func(types.NamespacedName)) psbinding.BindableContext {
	r := resolver.NewURIResolverFromTracker(ctx, tracker.New(handler, controller.GetTrackerLease(ctx)))

	return func(ctx context.Context, b psbinding.Bindable) (context.Context, error) {
		return v1.WithURIResolver(ctx, r), nil
	}
}

func (s *SinkBindingSubResourcesReconciler) Reconcile(ctx context.Context, b psbinding.Bindable) error {
	sb := b.(*v1.SinkBinding)
	if s.res == nil {
		err := errors.New("Resolver is nil")
		logging.FromContext(ctx).Errorf("%w", err)
		sb.Status.MarkBindingUnavailable("NoResolver", "No Resolver associated with context for sink")
		return err
	}
	if sb.Spec.Sink.Ref != nil {
		s.tracker.TrackReference(tracker.Reference{
			APIVersion: sb.Spec.Sink.Ref.APIVersion,
			Kind:       sb.Spec.Sink.Ref.Kind,
			Namespace:  sb.Spec.Sink.Ref.Namespace,
			Name:       sb.Spec.Sink.Ref.Name,
		}, b)
	}
	uri, err := s.res.URIFromDestinationV1(ctx, sb.Spec.Sink, sb)
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to get URI from Destination: %w", err)
		sb.Status.MarkBindingUnavailable("NoURI", "URI could not be extracted from destination")
		return err
	}
	sb.Status.MarkSink(uri)
	return nil
}

// I'm just here so I won't get fined
func (*SinkBindingSubResourcesReconciler) ReconcileDeletion(ctx context.Context, b psbinding.Bindable) error {
	return nil
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
