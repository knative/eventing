/*
Copyright 2019 The Knative Authors

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

package trigger

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/pkg/client/injection/informers/eventing/v1/broker"
	"knative.dev/eventing/pkg/client/injection/informers/eventing/v1/trigger"
	triggerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/trigger"
	"knative.dev/eventing/pkg/reconciler/sugar"
)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	triggerInformer := trigger.Get(ctx)
	brokerInformer := broker.Get(ctx)

	r := &Reconciler{
		eventingClientSet: eventingclient.Get(ctx),
		brokerLister:      brokerInformer.Lister(),
		isEnabled:         sugar.LabelFilterFnOrDie(ctx),
	}
	impl := triggerreconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			SkipStatusUpdates: true,
		}
	})

	logging.FromContext(ctx).Info("Setting up event handlers")
	triggerInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Watch brokers.
	brokerInformer.Informer().AddEventHandler(controller.HandleAll(func(obj interface{}) {
		if b, ok := obj.(*v1beta1.Broker); ok {
			triggers, err := triggerInformer.Lister().Triggers(b.Namespace).List(labels.SelectorFromSet(map[string]string{eventing.BrokerLabelKey: b.Name}))
			if err != nil {
				logging.FromContext(ctx).Warnw("Failed to list triggers", zap.String("Namespace", b.Namespace), zap.String("Broker", b.Name))
				return
			}
			for _, t := range triggers {
				impl.Enqueue(t)
			}
		}
	}))
	// When brokers change, change perform a global resync on triggers.
	grCb := func(obj interface{}) {
		logging.FromContext(ctx).Info("Doing a global resync on Triggers due to Brokers changing.")
		impl.GlobalResync(triggerInformer.Informer())
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
