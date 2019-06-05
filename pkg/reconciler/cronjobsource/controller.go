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
	"github.com/knative/eventing/pkg/apis/sources/v1alpha1"
	eventinginformers "github.com/knative/eventing/pkg/client/informers/externalversions/eventing/v1alpha1"
	sourceinformers "github.com/knative/eventing/pkg/client/informers/externalversions/sources/v1alpha1"
	"github.com/knative/eventing/pkg/duck"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/controller"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/tools/cache"
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
	opt reconciler.Options,
	cronjobsourceInformer sourceinformers.CronJobSourceInformer,
	deploymentInformer appsv1informers.DeploymentInformer,
	eventTypeInformer eventinginformers.EventTypeInformer,
) *controller.Impl {
	r := &Reconciler{
		Base:             reconciler.NewBase(opt, controllerAgentName),
		cronjobLister:    cronjobsourceInformer.Lister(),
		deploymentLister: deploymentInformer.Lister(),
		eventTypeLister:  eventTypeInformer.Lister(),
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName)
	r.sinkReconciler = duck.NewSinkReconciler(opt, impl.EnqueueKey)

	r.Logger.Info("Setting up event handlers")
	cronjobsourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

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
