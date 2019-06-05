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

package namespace

import (
	"github.com/knative/pkg/tracker"

	eventinginformers "github.com/knative/eventing/pkg/client/informers/externalversions/eventing/v1alpha1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	rbacv1informers "k8s.io/client-go/informers/rbac/v1"

	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/controller"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Namespace" // TODO: Namespace is not a very good name for this controller.

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "knative-eventing-namespace-controller"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	opt reconciler.Options,
	namespaceInformer corev1informers.NamespaceInformer,
	serviceAccountInformer corev1informers.ServiceAccountInformer,
	roleBindingInformer rbacv1informers.RoleBindingInformer,
	brokerInformer eventinginformers.BrokerInformer,
) *controller.Impl {

	r := &Reconciler{
		Base:            reconciler.NewBase(opt, controllerAgentName),
		namespaceLister: namespaceInformer.Lister(),
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName)
	// TODO: filter label selector: on InjectionEnabledLabels()

	r.Logger.Info("Setting up event handlers")
	namespaceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Tracker is used to notify us the namespace's resources we need to reconcile.
	r.tracker = tracker.New(impl.EnqueueKey, opt.GetTrackerLease())

	// Watch all the resources that this reconciler reconciles.
	serviceAccountInformer.Informer().AddEventHandler(controller.HandleAll(
		controller.EnsureTypeMeta(r.tracker.OnChanged, serviceAccountGVK),
	))
	roleBindingInformer.Informer().AddEventHandler(controller.HandleAll(
		controller.EnsureTypeMeta(r.tracker.OnChanged, roleBindingGVK),
	))
	brokerInformer.Informer().AddEventHandler(controller.HandleAll(
		controller.EnsureTypeMeta(r.tracker.OnChanged, brokerGVK),
	))

	return impl
}
