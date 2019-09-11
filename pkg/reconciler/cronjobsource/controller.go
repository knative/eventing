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
	"fmt"

	"github.com/kelseyhightower/envconfig"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	eventtypeinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventtype"
	cronjobsourceinformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1alpha1/cronjobsource"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "CronJobSources"
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cronjob-source-controller"
)

// envConfig will be used to extract the required environment variables using
// github.com/kelseyhightower/envconfig. If this configuration cannot be extracted, then
// NewController will panic.
type envConfig struct {
	Image string `envconfig:"CRONJOB_RA_IMAGE" required:"true"`
}

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	deploymentInformer := deploymentinformer.Get(ctx)
	cronJobSourceInformer := cronjobsourceinformer.Get(ctx)
	eventTypeInformer := eventtypeinformer.Get(ctx)

	env := &envConfig{}
	if err := envconfig.Process("", env); err != nil {
		panic(fmt.Errorf("unable to process CronJobSource's required environment variables: %v", err))
	}

	r := &Reconciler{
		Base:             reconciler.NewBase(ctx, controllerAgentName, cmw),
		cronjobLister:    cronJobSourceInformer.Lister(),
		deploymentLister: deploymentInformer.Lister(),
		eventTypeLister:  eventTypeInformer.Lister(),
		env:              *env,
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName)
	r.sinkReconciler = duck.NewSinkReconciler(ctx, impl.EnqueueKey)

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
