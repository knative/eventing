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

package duck

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/pkg/apis/duck"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/logging"

	eventtypeinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventtype"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	crdinfomer "knative.dev/pkg/client/injection/apiextensions/informers/apiextensions/v1beta1/customresourcedefinition"
)

const (
	// ReconcilerName is the name of the reconciler.
	ReconcilerName = "SourceDucks"
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "source-duck-controller"
)

// NewController returns a function that initializes the controller and
// Registers event handlers to enqueue events
func NewController(crd string, gvr schema.GroupVersionResource, gvk schema.GroupVersionKind) injection.ControllerConstructor {
	return func(ctx context.Context,
		cmw configmap.Watcher,
	) *controller.Impl {

		eventTypeInformer := eventtypeinformer.Get(ctx)
		crdInformer := crdinfomer.Get(ctx)

		// Create a duck TypedInformer for duckv1.Source resources.
		sourceinformer := &duck.TypedInformerFactory{
			Client:       dynamicclient.Get(ctx),
			Type:         &duckv1.Source{},
			ResyncPeriod: controller.DefaultResyncPeriod,
			StopChannel:  ctx.Done(),
		}

		sourceInformer, sourceLister, err := sourceinformer.Get(gvr)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Error getting source informer", zap.String("GVR", gvr.String()), zap.Error(err))
			return nil
		}

		r := &Reconciler{
			Base:            reconciler.NewBase(ctx, controllerAgentName, cmw),
			eventTypeLister: eventTypeInformer.Lister(),
			crdLister:       crdInformer.Lister(),
			sourceLister:    sourceLister,
			gvr:             gvr,
			crdName:         crd,
		}
		impl := controller.NewImpl(r, r.Logger, ReconcilerName)

		r.Logger.Info("Setting up event handlers")
		sourceInformer.AddEventHandler(controller.HandleAll(impl.Enqueue))

		eventTypeInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterGroupVersionKind(gvk),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		return impl
	}
}
