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

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/logging"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "knative-eventing-namespace-controller"

	// Label to enable knative-eventing in a namespace.
	knativeEventingLabelKey   = "knative-eventing-injection"
	knativeEventingLabelValue = "enabled"

	defaultBroker           = "default"
	brokerFilterSA          = "eventing-broker-filter"
	brokerFilterRB          = "eventing-broker-filter"
	brokerFilterClusterRole = "eventing-broker-filter"

	// Name of the corev1.Events emitted from the reconciliation process.
	brokerCreated             = "BrokerCreated"
	serviceAccountCreated     = "BrokerFilterServiceAccountCreated"
	serviceAccountRBACCreated = "BrokerFilterServiceAccountRBACCreated"
)

type reconciler struct {
	client   client.Client
	recorder record.EventRecorder

	logger *zap.Logger
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

// ProvideController returns a function that returns a Namespace controller.
func ProvideController(mgr manager.Manager, logger *zap.Logger) (controller.Controller, error) {
	// Setup a new controller to Reconcile Namespaces.
	r := &reconciler{
		recorder: mgr.GetRecorder(controllerAgentName),
		logger:   logger,
	}
	c, err := controller.New(controllerAgentName, mgr, controller.Options{
		Reconciler: r,
	})
	if err != nil {
		return nil, err
	}

	// Watch Namespaces.
	if err = c.Watch(&source.Kind{Type: &v1.Namespace{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}

	// Watch all the resources that this reconciler reconciles. This is a map from resource type to
	// the name of the resource of that type we care about (i.e. only if the resource of the given
	// type and with the given name changes, do we reconcile the Namespace).
	resources := map[runtime.Object]string{
		&corev1.ServiceAccount{}: brokerFilterSA,
		&rbacv1.RoleBinding{}:    brokerFilterRB,
		&v1alpha1.Broker{}:       defaultBroker,
	}
	for t, n := range resources {
		nm := &namespaceMapper{
			name: n,
		}
		err = c.Watch(&source.Kind{Type: t}, &handler.EnqueueRequestsFromMapFunc{ToRequests: nm})
		if err != nil {
			return nil, err
		}
	}

	return c, nil
}

type namespaceMapper struct {
	name string
}

var _ handler.Mapper = &namespaceMapper{}

func (m *namespaceMapper) Map(o handler.MapObject) []reconcile.Request {
	if o.Meta.GetName() == m.name {
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Namespace: "",
					Name:      o.Meta.GetNamespace(),
				},
			},
		}
	}
	return []reconcile.Request{}
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Namespace resource
// with the current status of the resource.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	ctx = logging.WithLogger(ctx, r.logger.With(zap.Any("request", request)))

	ns := &corev1.Namespace{}
	err := r.client.Get(ctx, request.NamespacedName, ns)

	if errors.IsNotFound(err) {
		logging.FromContext(ctx).Info("Could not find Namespace")
		return reconcile.Result{}, nil
	}

	if err != nil {
		logging.FromContext(ctx).Error("Could not Get Namespace", zap.Error(err))
		return reconcile.Result{}, err
	}

	if ns.Labels[knativeEventingLabelKey] != knativeEventingLabelValue {
		logging.FromContext(ctx).Debug("Not reconciling Namespace")
		return reconcile.Result{}, nil
	}

	// Reconcile this copy of the Namespace and then write back any status updates regardless of
	// whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, ns)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling Namespace", zap.Error(reconcileErr))
	} else {
		logging.FromContext(ctx).Debug("Namespace reconciled")
	}

	// Requeue if the resource is not ready:
	return reconcile.Result{}, reconcileErr
}

func (r *reconciler) reconcile(ctx context.Context, ns *corev1.Namespace) error {
	// No need for a finalizer, because everything reconciled is created inside the Namespace. If
	// the Namespace is being deleted, then all the reconciled objects will be too.

	if ns.DeletionTimestamp != nil {
		return nil
	}

	sa, err := r.reconcileBrokerFilterServiceAccount(ctx, ns)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to reconcile the Broker Filter Service Account for the namespace", zap.Error(err))
		return err
	}
	_, err = r.reconcileBrokerFilterRBAC(ctx, ns, sa)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to reconcile the Broker Filter Service Account RBAC for the namespace", zap.Error(err))
		return err
	}
	_, err = r.reconcileBroker(ctx, ns)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to reconcile Broker for the namespace", zap.Error(err))
		return err
	}

	return nil
}

// reconcileBrokerFilterServiceAccount reconciles the Broker's filter service account for Namespace 'ns'.
func (r *reconciler) reconcileBrokerFilterServiceAccount(ctx context.Context, ns *corev1.Namespace) (*corev1.ServiceAccount, error) {
	current, err := r.getBrokerFilterServiceAccount(ctx, ns)

	// If the resource doesn't exist, we'll create it.
	if k8serrors.IsNotFound(err) {
		sa := newBrokerFilterServiceAccount(ns)
		err = r.client.Create(ctx, sa)
		if err != nil {
			return nil, err
		}
		r.recorder.Event(ns,
			corev1.EventTypeNormal,
			serviceAccountCreated,
			fmt.Sprintf("Service account created for the Broker '%s'", sa.Name))
		return sa, nil
	} else if err != nil {
		return nil, err
	}
	// Don't update anything that is already present.
	return current, nil
}

// getBrokerFilterServiceAccount returns the Broker's filter service account for Namespace 'ns' if exists,
// otherwise it returns an error.
func (r *reconciler) getBrokerFilterServiceAccount(ctx context.Context, ns *corev1.Namespace) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{}
	name := types.NamespacedName{
		Namespace: ns.Name,
		Name:      brokerFilterSA,
	}
	err := r.client.Get(ctx, name, sa)
	return sa, err
}

// newBrokerFilterServiceAccount creates a ServiceAccount object for the Namespace 'ns'.
func newBrokerFilterServiceAccount(ns *corev1.Namespace) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns.Name,
			Name:      brokerFilterSA,
			Labels:    injectedLabels(),
		},
	}
}

func injectedLabels() map[string]string {
	return map[string]string{
		"eventing.knative.dev/namespaceInjected": "true",
	}
}

// reconcileBrokerFilterRBAC reconciles the Broker's filter service account RBAC for the Namespace 'ns'.
func (r *reconciler) reconcileBrokerFilterRBAC(ctx context.Context, ns *corev1.Namespace, sa *corev1.ServiceAccount) (*rbacv1.RoleBinding, error) {
	current, err := r.getBrokerFilterRBAC(ctx, ns)

	// If the resource doesn't exist, we'll create it.
	if k8serrors.IsNotFound(err) {
		rbac := newBrokerFilterRBAC(ns, sa)
		err = r.client.Create(ctx, rbac)
		if err != nil {
			return nil, err
		}
		r.recorder.Event(ns,
			corev1.EventTypeNormal,
			serviceAccountRBACCreated,
			fmt.Sprintf("Service account RBAC created for the Broker Filter '%s'", rbac.Name))
		return rbac, nil
	} else if err != nil {
		return nil, err
	}
	// Don't update anything that is already present.
	return current, nil
}

// getBrokerFilterRBAC returns the Broker's filter role binding for Namespace 'ns' if exists,
// otherwise it returns an error.
func (r *reconciler) getBrokerFilterRBAC(ctx context.Context, ns *corev1.Namespace) (*rbacv1.RoleBinding, error) {
	rb := &rbacv1.RoleBinding{}
	name := types.NamespacedName{
		Namespace: ns.Name,
		Name:      brokerFilterRB,
	}
	err := r.client.Get(ctx, name, rb)
	return rb, err
}

// newBrokerFilterRBAC creates a RpleBinding object for the Broker's filter service account 'sa' in the Namespace 'ns'.
func newBrokerFilterRBAC(ns *corev1.Namespace, sa *corev1.ServiceAccount) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns.Name,
			Name:      brokerFilterRB,
			Labels:    injectedLabels(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     brokerFilterClusterRole,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: ns.Name,
				Name:      sa.Name,
			},
		},
	}
}

// getBroker returns the default broker for Namespace 'ns' if it exists, otherwise it returns an
// error.
func (r *reconciler) getBroker(ctx context.Context, ns *corev1.Namespace) (*v1alpha1.Broker, error) {
	b := &v1alpha1.Broker{}
	name := types.NamespacedName{
		Namespace: ns.Name,
		Name:      defaultBroker,
	}
	err := r.client.Get(ctx, name, b)
	return b, err
}

// reconcileBroker reconciles the default Broker for the Namespace 'ns'.
func (r *reconciler) reconcileBroker(ctx context.Context, ns *corev1.Namespace) (*v1alpha1.Broker, error) {
	current, err := r.getBroker(ctx, ns)

	// If the resource doesn't exist, we'll create it.
	if k8serrors.IsNotFound(err) {
		b := newBroker(ns)
		err = r.client.Create(ctx, b)
		if err != nil {
			return nil, err
		}
		r.recorder.Event(ns, corev1.EventTypeNormal, brokerCreated, "Default eventing.knative.dev Broker created.")
		return b, nil
	} else if err != nil {
		return nil, err
	}
	// Don't update anything that is already present.
	return current, nil
}

// newBroker creates a placeholder default Broker object for Namespace 'ns'.
func newBroker(ns *corev1.Namespace) *v1alpha1.Broker {
	return &v1alpha1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns.Name,
			Name:      defaultBroker,
			Labels:    injectedLabels(),
		},
	}
}
