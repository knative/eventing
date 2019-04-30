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
	controllerAgentName = "knative-eventing-namespace-controller"

	// Name of the corev1.Events emitted from the reconciliation process.
	brokerCreated             = "BrokerCreated"
	serviceAccountCreated     = "BrokerFilterServiceAccountCreated"
	serviceAccountRBACCreated = "BrokerFilterServiceAccountRBACCreated"
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
	impl := controller.NewImpl(r, r.Logger, ReconcilerName, reconciler.MustNewStatsReporter(ReconcilerName, r.Logger))

	// TODO: filter label selector: on InjectionEnabledLabels()

	r.Logger.Info("Setting up event handlers")
	namespaceInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	// Tracker is used to notify us the namespace's resources we need to reconcile.
	r.tracker = tracker.New(impl.EnqueueKey, opt.GetTrackerLease())

	// Watch all the resources that this reconciler reconciles.
	serviceAccountInformer.Informer().AddEventHandler(reconciler.Handler(
		controller.EnsureTypeMeta(r.tracker.OnChanged, serviceAccountGVK),
	))
	roleBindingInformer.Informer().AddEventHandler(reconciler.Handler(
		controller.EnsureTypeMeta(r.tracker.OnChanged, roleBindingGVK),
	))
	brokerInformer.Informer().AddEventHandler(reconciler.Handler(
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
		r.Logger.Errorf("invalid resource key: %s", key)
		return nil
	}

	// Get the namespace resource with this namespace/name
	original, err := r.namespaceLister.Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("namespace key in work queue no longer exists", zap.Any("key", key))
		return nil
	} else if err != nil {
		return err
	}

	if original.Labels[resources.InjectionLabelKey] != resources.InjectionEnabledLabelValue {
		logging.FromContext(ctx).Debug("Not reconciling Namespace")
		// TODO: this does not handle cleanup of unwanted brokers in namespace.
		return nil
	}

	// Don't modify the informers copy
	ns := original.DeepCopy()

	// Reconcile this copy of the Namespace and then write back any status updates regardless of
	// whether the reconcile error out.
	err = r.reconcile(ctx, ns)
	if err != nil {
		logging.FromContext(ctx).Error("Error reconciling Namespace", zap.Error(err), zap.Any("key", key))
	} else {
		logging.FromContext(ctx).Debug("Namespace reconciled", zap.Any("key", key))
	}

	// Requeue if the resource is not ready:
	return err
}

func (r *Reconciler) reconcile(ctx context.Context, ns *corev1.Namespace) error {
	if ns.DeletionTimestamp != nil {
		return nil
	}
	sa, err := r.reconcileBrokerFilterServiceAccount(ctx, ns)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to reconcile the Broker Filter Service Account for the namespace", zap.Error(err))
		return err
	}

	// Tell tracker to reconcile this namespace whenever the Service Account changes.
	if err = r.tracker.Track(utils.ObjectRef(sa, serviceAccountGVK), ns); err != nil {
		logging.FromContext(ctx).Error("Unable to track changes to ServiceAccount", zap.Error(err))
		return err
	}

	rb, err := r.reconcileBrokerFilterRBAC(ctx, ns, sa)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to reconcile the Broker Filter Service Account RBAC for the namespace", zap.Error(err))
		return err
	}

	// Tell tracker to reconcile this namespace whenever the RoleBinding changes.
	if err = r.tracker.Track(utils.ObjectRef(rb, roleBindingGVK), ns); err != nil {
		logging.FromContext(ctx).Error("Unable to track changes to RoleBinding", zap.Error(err))
		return err
	}

	b, err := r.reconcileBroker(ctx, ns)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to reconcile Broker for the namespace", zap.Error(err))
		return err
	}

	// Tell tracker to reconcile this namespace whenever the Broker changes.
	if err = r.tracker.Track(utils.ObjectRef(b, brokerGVK), ns); err != nil {
		logging.FromContext(ctx).Error("Unable to track changes to Broker", zap.Error(err))
		return err
	}

	return nil
}

// reconcileBrokerFilterServiceAccount reconciles the Broker's filter service account for Namespace 'ns'.
func (r *Reconciler) reconcileBrokerFilterServiceAccount(ctx context.Context, ns *corev1.Namespace) (*corev1.ServiceAccount, error) {
	current, err := r.KubeClientSet.CoreV1().ServiceAccounts(ns.Name).Get(resources.ServiceAccountName, metav1.GetOptions{})

	// If the resource doesn't exist, we'll create it.
	if k8serrors.IsNotFound(err) {
		sa := resources.MakeServiceAccount(ns.Name)
		sa, err := r.KubeClientSet.CoreV1().ServiceAccounts(ns.Name).Create(sa)
		if err != nil {
			return nil, err
		}
		r.Recorder.Event(ns, corev1.EventTypeNormal, serviceAccountCreated,
			fmt.Sprintf("Service account created for the Broker '%s'", sa.Name))
		return sa, nil
	} else if err != nil {
		return nil, err
	}
	// Don't update anything that is already present.
	return current, nil
}

// reconcileBrokerFilterRBAC reconciles the Broker's filter service account RBAC for the Namespace 'ns'.
func (r *Reconciler) reconcileBrokerFilterRBAC(ctx context.Context, ns *corev1.Namespace, sa *corev1.ServiceAccount) (*rbacv1.RoleBinding, error) {
	current, err := r.KubeClientSet.RbacV1().RoleBindings(ns.Name).Get(resources.RoleBindingName, metav1.GetOptions{})

	// If the resource doesn't exist, we'll create it.
	if k8serrors.IsNotFound(err) {
		rb := resources.MakeRoleBinding(sa)
		rb, err := r.KubeClientSet.RbacV1().RoleBindings(ns.Name).Create(rb)
		if err != nil {
			return nil, err
		}
		r.Recorder.Event(ns, corev1.EventTypeNormal, serviceAccountRBACCreated,
			fmt.Sprintf("Service account RBAC created for the Broker Filter '%s'", rb.Name))
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
