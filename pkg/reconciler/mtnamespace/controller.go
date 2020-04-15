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

package mtnamespace

import (
	"context"

	"github.com/kelseyhightower/envconfig"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/pkg/reconciler/mtnamespace/resources"
	namespacereconciler "knative.dev/pkg/client/injection/kube/reconciler/core/v1/namespace"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/client/injection/informers/eventing/v1beta1/broker"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/namespace"
)

type envConfig struct {
	InjectionDefault bool `envconfig:"BROKER_INJECTION_DEFAULT" default:"true"`
}

func onByDefault(labels map[string]string) bool {
	return labels[resources.InjectionLabelKey] == resources.InjectionDisabledLabelValue
}

func offByDefault(labels map[string]string) bool {
	return labels[resources.InjectionLabelKey] != resources.InjectionEnabledLabelValue
}

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	namespaceInformer := namespace.Get(ctx)
	brokerInformer := broker.Get(ctx)

	var filter labelFilter

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logging.FromContext(ctx).Fatalf("mtnamespace was unable to process environment: %v", err)
	} else if env.InjectionDefault {
		filter = onByDefault
	} else {
		filter = offByDefault
	}

	r := &Reconciler{
		eventingClientSet: eventingclient.Get(ctx),
		filter:            filter,
		brokerLister:      brokerInformer.Lister(),
	}

	impl := namespacereconciler.NewImpl(ctx, r)
	// TODO: filter label selector: on InjectionEnabledLabels()

	logging.FromContext(ctx).Info("Setting up event handlers")
	namespaceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))
	brokerInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Namespace")),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

	return impl
}
