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

package cronjobsource

import (
	"context"

	"github.com/knative/eventing/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing/pkg/duck"
	"github.com/knative/eventing/pkg/reconciler"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"k8s.io/client-go/tools/cache"

	eventtypeinformer "github.com/knative/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventtype"
	cronjobsourceinformer "github.com/knative/eventing/pkg/client/injection/informers/sources/v1alpha1/cronjobsource"
	deploymentinformer "knative.dev/pkg/injection/informers/kubeinformers/appsv1/deployment"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "CronJobSources"
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cronjob-source-controller"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	deploymentInformer := deploymentinformer.Get(ctx)
	cronJobSourceInformer := cronjobsourceinformer.Get(ctx)
	eventTypeInformer := eventtypeinformer.Get(ctx)

	r := &Reconciler{
		Base:             reconciler.NewBase(ctx, controllerAgentName, cmw),
		cronjobLister:    cronJobSourceInformer.Lister(),
		deploymentLister: deploymentInformer.Lister(),
		eventTypeLister:  eventTypeInformer.Lister(),
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName)
	r.sinkReconciler = duck.NewInjectionSinkReconciler(ctx, impl.EnqueueKey)

	r.Logger.Info("Setting up event handlers")
	cronJobSourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("CronJobSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	eventTypeInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("CronJobSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
