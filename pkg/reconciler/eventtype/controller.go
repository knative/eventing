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

package eventtype

import (
	"context"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	brokerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/broker"
	eventtypeinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1beta1/eventtype"
	eventtypereconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1beta1/eventtype"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
// TODO remove https://github.com/knative/eventing/issues/2750
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	brokerInformer := brokerinformer.Get(ctx)
	eventTypeInformer := eventtypeinformer.Get(ctx)

	r := &Reconciler{
		eventTypeLister: eventTypeInformer.Lister(),
		brokerLister:    brokerInformer.Lister(),
	}
	impl := eventtypereconciler.NewImpl(ctx, r)

	eventTypeInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Tracker is used to notify us that a EventType's Broker has changed so that
	// we can reconcile.
	r.tracker = impl.Tracker
	brokerInformer.Informer().AddEventHandler(controller.HandleAll(
		controller.EnsureTypeMeta(
			r.tracker.OnChanged,
			v1.SchemeGroupVersion.WithKind("Broker"),
		),
	))

	return impl
}
