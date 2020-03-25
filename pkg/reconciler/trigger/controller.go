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

	"knative.dev/eventing/pkg/logging"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/broker"
	"knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/trigger"
	triggerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1alpha1/trigger"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/namespace"
)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	triggerInformer := trigger.Get(ctx)
	brokerInformer := broker.Get(ctx)
	namespaceInformer := namespace.Get(ctx)

	r := &Reconciler{
		eventingClientSet: eventingclient.Get(ctx),
		kubeClientSet:     kubeclient.Get(ctx),
		brokerLister:      brokerInformer.Lister(),
		namespaceLister:   namespaceInformer.Lister(),
	}
	impl := triggerreconciler.NewImpl(ctx, r)

	logging.FromContext(ctx).Info("Setting up event handlers")
	triggerInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))
	return impl
}
