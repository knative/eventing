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

package requestreply

import (
	"context"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	statefulsetinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/statefulset"
	secretinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/secret/filtered"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/client/injection/informers/eventing/v1/broker"
	"knative.dev/eventing/pkg/client/injection/informers/eventing/v1/trigger"
	"knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/requestreply"
	requestreplyreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1alpha1/requestreply"
	eventingv1alpha1listers "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	requestReplyInformer := requestreply.Get(ctx)
	brokerInformer := broker.Get(ctx)
	statefulSetInformer := statefulsetinformer.Get(ctx)
	triggerInformer := trigger.Get(ctx)
	logger := logging.FromContext(ctx)

	r := &Reconciler{
		kubeClient:        kubeclient.Get(ctx),
		eventingClient:    eventingclient.Get(ctx),
		secretLister:      secretinformer.Get(ctx, SecretLabelSelector).Lister(),
		triggerLister:     triggerInformer.Lister(),
		brokerLister:      brokerInformer.Lister(),
		statefulSetLister: statefulSetInformer.Lister(),
		deleteContext:     ctx,
	}

	impl := requestreplyreconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		return controller.Options{}
	})

	requestReplyInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	statefulSetInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj any) bool {
			ss := obj.(*appsv1.StatefulSet)
			return ss.GetNamespace() == system.Namespace() && ss.GetName() == statefulSetName
		},
		Handler: controller.HandleAll(func(_ any) {
			impl.GlobalResync(requestReplyInformer.Informer())
		}),
	})

	triggerInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj any) bool {
			trigger := obj.(*eventingv1.Trigger)
			_, ok := trigger.Labels[triggerNameLabelKey] // every trigger associated with a request reply will have this label
			logger.Info("trigger had label", zap.Bool("hadLabel", ok), zap.Any("labels", trigger.Labels))
			return ok
		},
		Handler: enqueueRequestRepliesForTrigger(requestReplyInformer.Lister(), impl.Enqueue),
	})

	brokerInformer.Informer().AddEventHandler(enqueueRequestRepliesForBroker(requestReplyInformer.Lister(), impl.Enqueue))

	return impl
}

func enqueueRequestRepliesForTrigger(
	lister eventingv1alpha1listers.RequestReplyLister,
	enqueue func(obj any),
) cache.ResourceEventHandler {
	return controller.HandleAll(func(obj any) {
		if trigger, ok := obj.(*eventingv1.Trigger); ok {
			if rrName, ok := trigger.Labels[triggerNameLabelKey]; ok {
				rr, err := lister.RequestReplies(trigger.Namespace).Get(rrName)
				if err != nil {
					return
				}

				enqueue(rr)
			}
		}
	})
}

func enqueueRequestRepliesForBroker(
	lister eventingv1alpha1listers.RequestReplyLister,
	enqueue func(obj any),
) cache.ResourceEventHandler {
	return controller.HandleAll(func(obj any) {
		if broker, ok := obj.(*eventingv1.Broker); ok {
			requestReplies, err := lister.RequestReplies(broker.Namespace).List(
				labels.SelectorFromSet(map[string]string{"eventing.knative.dev/broker": broker.Name}),
			)
			if err != nil {
				return
			}
			for rr := range requestReplies {
				enqueue(rr)
			}
		}
	})
}
