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
	"context"
	"fmt"

	"github.com/knative/eventing/pkg/reconciler/namespace/resources"
	"github.com/knative/eventing/pkg/utils"
	"github.com/knative/pkg/tracker"
	"k8s.io/client-go/tools/cache"

	eventinginformers "github.com/knative/eventing/pkg/client/informers/externalversions/eventing/v1alpha1"
	eventinglisters "github.com/knative/eventing/pkg/client/listers/eventing/v1alpha1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	rbacv1informers "k8s.io/client-go/informers/rbac/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/controller"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Namespace" // TODO: Namespace is not a very good name for this controller.

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName       = "knative-eventing-namespace-controller"
	namespaceReconciled       = "NamespaceReconciled"
	namespaceReconcileFailure = "NamespaceReconcileFailure"

	// Name of the corev1.Events emitted from the reconciliation process.
	brokerCreated             = "BrokerCreated"
	serviceAccountCreated     = "BrokerServiceAccountCreated"
	serviceAccountRBACCreated = "BrokerServiceAccountRBACCreated"
)

var (
	serviceAccountGVK = corev1.SchemeGroupVersion.WithKind("ServiceAccount")
	roleBindingGVK    = rbacv1.SchemeGroupVersion.WithKind("RoleBinding")
	brokerGVK         = v1alpha1.SchemeGroupVersion.WithKind("Broker")
)

type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	namespaceLister      corev1listers.NamespaceLister
	serviceAccountLister corev1listers.ServiceAccountLister
	roleBindingLister    rbacv1listers.RoleBindingLister
	brokerLister         eventinglisters.BrokerLister
	tracker              tracker.Interface
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

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

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Namespace resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the namespace resource with this namespace/name
	original, err := r.namespaceLister.Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("namespace key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	if original.Labels[resources.InjectionLabelKey] != resources.InjectionEnabledLabelValue {
		logging.FromContext(ctx).Debug("Not reconciling Namespace")
		return nil
	}

	// Don't modify the informers copy
	ns := original.DeepCopy()

	// Reconcile this copy of the Namespace.
	reconcileErr := r.reconcile(ctx, ns)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling Namespace", zap.Error(reconcileErr))
		r.Recorder.Eventf(ns, corev1.EventTypeWarning, namespaceReconcileFailure, "Failed to reconcile Namespace: %v", reconcileErr)
	} else {
		logging.FromContext(ctx).Debug("Namespace reconciled")
		r.Recorder.Eventf(ns, corev1.EventTypeNormal, namespaceReconciled, "Namespace reconciled: %q", ns.Name)
	}

	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, ns *corev1.Namespace) error {
	if ns.DeletionTimestamp != nil {
		return nil
	}
	if err := r.reconcileServiceAccountAndRoleBinding(ctx, ns, resources.IngressServiceAccountName, resources.IngressRoleBindingName, resources.IngressClusterRoleName); err != nil {
		return fmt.Errorf("broker ingress: %v", err)
	}
	if err := r.reconcileServiceAccountAndRoleBinding(ctx, ns, resources.FilterServiceAccountName, resources.FilterRoleBindingName, resources.FilterClusterRoleName); err != nil {
		return fmt.Errorf("broker filter: %v", err)
	}

	b, err := r.reconcileBroker(ctx, ns)
	if err != nil {
		return fmt.Errorf("broker: %v", err)
	}

	// Tell tracker to reconcile this namespace whenever the Broker changes.
	if err = r.tracker.Track(utils.ObjectRef(b, brokerGVK), ns); err != nil {
		return fmt.Errorf("track broker: %v", err)
	}

	return nil
}

// reconcileServiceAccountAndRoleBinding reconciles the service account and role binding for
// Namespace 'ns'.
func (r *Reconciler) reconcileServiceAccountAndRoleBinding(ctx context.Context, ns *corev1.Namespace, saName, rbName, clusterRoleName string) error {
	sa, err := r.reconcileBrokerServiceAccount(ctx, ns, resources.MakeServiceAccount(ns.Name, saName))
	if err != nil {
		return fmt.Errorf("service account '%s': %v", saName, err)
	}

	// Tell tracker to reconcile this namespace whenever the Service Account changes.
	if err = r.tracker.Track(utils.ObjectRef(sa, serviceAccountGVK), ns); err != nil {
		return fmt.Errorf("track service account '%s': %v", sa.Name, err)
	}

	rb, err := r.reconcileBrokerRBAC(ctx, ns, sa, resources.MakeRoleBinding(rbName, sa, clusterRoleName))
	if err != nil {
		return fmt.Errorf("role binding '%s': %v", rbName, err)
	}

	// Tell tracker to reconcile this namespace whenever the RoleBinding changes.
	if err = r.tracker.Track(utils.ObjectRef(rb, roleBindingGVK), ns); err != nil {
		return fmt.Errorf("track role binding '%s': %v", rb.Name, err)
	}

	return nil
}

// reconcileBrokerServiceAccount reconciles the Broker's service account for Namespace 'ns'.
func (r *Reconciler) reconcileBrokerServiceAccount(ctx context.Context, ns *corev1.Namespace, sa *corev1.ServiceAccount) (*corev1.ServiceAccount, error) {
	current, err := r.KubeClientSet.CoreV1().ServiceAccounts(ns.Name).Get(sa.Name, metav1.GetOptions{})

	// If the resource doesn't exist, we'll create it.
	if k8serrors.IsNotFound(err) {
		sa, err = r.KubeClientSet.CoreV1().ServiceAccounts(ns.Name).Create(sa)
		if err != nil {
			return nil, err
		}
		r.Recorder.Event(ns, corev1.EventTypeNormal, serviceAccountCreated,
			fmt.Sprintf("Service account '%s' created for the Broker", sa.Name))
		return sa, nil
	} else if err != nil {
		return nil, err
	}
	// Don't update anything that is already present.
	return current, nil
}

// reconcileBrokerRBAC reconciles the Broker's service account RBAC for the Namespace 'ns'.
func (r *Reconciler) reconcileBrokerRBAC(ctx context.Context, ns *corev1.Namespace, sa *corev1.ServiceAccount, rb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
	current, err := r.KubeClientSet.RbacV1().RoleBindings(ns.Name).Get(rb.Name, metav1.GetOptions{})

	// If the resource doesn't exist, we'll create it.
	if k8serrors.IsNotFound(err) {
		rb, err = r.KubeClientSet.RbacV1().RoleBindings(ns.Name).Create(rb)
		if err != nil {
			return nil, err
		}
		r.Recorder.Event(ns, corev1.EventTypeNormal, serviceAccountRBACCreated,
			fmt.Sprintf("Service account RBAC '%s' created for the Broker", rb.Name))
		return rb, nil
	} else if err != nil {
		return nil, err
	}
	// Don't update anything that is already present.
	return current, nil
}

// reconcileBroker reconciles the default Broker for the Namespace 'ns'.
func (r *Reconciler) reconcileBroker(ctx context.Context, ns *corev1.Namespace) (*v1alpha1.Broker, error) {
	current, err := r.EventingClientSet.EventingV1alpha1().Brokers(ns.Name).Get(resources.DefaultBrokerName, metav1.GetOptions{})

	// If the resource doesn't exist, we'll create it.
	if k8serrors.IsNotFound(err) {
		b := resources.MakeBroker(ns.Name)
		b, err = r.EventingClientSet.EventingV1alpha1().Brokers(ns.Name).Create(b)
		if err != nil {
			return nil, err
		}
		r.Recorder.Event(ns, corev1.EventTypeNormal, brokerCreated,
			"Default eventing.knative.dev Broker created.")
		return b, nil
	} else if err != nil {
		return nil, err
	}
	// Don't update anything that is already present.
	return current, nil
}
