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

	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/reconciler/namespace/resources"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/system"

	corev1listers "k8s.io/client-go/listers/core/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	configslisters "knative.dev/eventing/pkg/client/listers/configs/v1alpha1"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1beta1"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	configsv1alpha1 "knative.dev/eventing/pkg/apis/configs/v1alpha1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/pkg/controller"
)

const (
	namespaceReconciled       = "NamespaceReconciled"
	namespaceReconcileFailure = "NamespaceReconcileFailure"

	// Name of the corev1.Events emitted from the reconciliation process.
	configMapPropagationCreated = "ConfigMapPropagationCreated"
	brokerCreated               = "BrokerCreated"
	serviceAccountCreated       = "BrokerServiceAccountCreated"
	serviceAccountRBACCreated   = "BrokerServiceAccountRBACCreated"
	secretCopied                = "SecretCopied"
	secretCopyFailure           = "SecretCopyFailure"
)

var (
	serviceAccountGVK       = corev1.SchemeGroupVersion.WithKind("ServiceAccount")
	roleBindingGVK          = rbacv1.SchemeGroupVersion.WithKind("RoleBinding")
	brokerGVK               = v1alpha1.SchemeGroupVersion.WithKind("Broker")
	configMapPropagationGVK = configsv1alpha1.SchemeGroupVersion.WithKind("ConfigMapPropagation")
)

type Reconciler struct {
	*reconciler.Base

	brokerPullSecretName string

	// listers index properties about resources
	namespaceLister            corev1listers.NamespaceLister
	serviceAccountLister       corev1listers.ServiceAccountLister
	roleBindingLister          rbacv1listers.RoleBindingLister
	brokerLister               eventinglisters.BrokerLister
	configMapPropagationLister configslisters.ConfigMapPropagationLister
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

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

	if _, err := r.reconcileConfigMapPropagation(ctx, ns); err != nil {
		return fmt.Errorf("configMapPropagation: %w", err)
	}

	if err := r.reconcileServiceAccountAndRoleBindings(ctx, ns, resources.IngressServiceAccountName, resources.IngressRoleBindingName, resources.IngressClusterRoleName); err != nil {
		return fmt.Errorf("broker ingress: %w", err)
	}

	if err := r.reconcileServiceAccountAndRoleBindings(ctx, ns, resources.FilterServiceAccountName, resources.FilterRoleBindingName, resources.FilterClusterRoleName); err != nil {
		return fmt.Errorf("broker filter: %w", err)
	}

	if _, err := r.reconcileBroker(ctx, ns); err != nil {
		return fmt.Errorf("broker: %v", err)
	}

	return nil
}

// reconcileConfigMapPropagation reconciles the default ConfigMapPropagation for the Namespace 'ns'.
func (r *Reconciler) reconcileConfigMapPropagation(ctx context.Context, ns *corev1.Namespace) (*configsv1alpha1.ConfigMapPropagation, error) {
	current, err := r.configMapPropagationLister.ConfigMapPropagations(ns.Name).Get(resources.DefaultConfigMapPropagationName)

	// If the resource doesn't exist, we'll create it.
	if k8serrors.IsNotFound(err) {
		cmp := resources.MakeConfigMapPropagation(ns)
		cmp, err = r.EventingClientSet.ConfigsV1alpha1().ConfigMapPropagations(ns.Name).Create(cmp)
		if err != nil {
			return nil, err
		}
		r.Recorder.Event(ns, corev1.EventTypeNormal, configMapPropagationCreated,
			"Default ConfigMapPropagation: "+cmp.Name+" created")
		return cmp, nil
	} else if err != nil {
		return nil, err
	}
	// Don't update anything that is already present.
	return current, nil
}

// reconcileServiceAccountAndRoleBinding reconciles the service account and role binding for
// Namespace 'ns'.
func (r *Reconciler) reconcileServiceAccountAndRoleBindings(ctx context.Context, ns *corev1.Namespace, saName, rbName, clusterRoleName string) error {
	sa, err := r.reconcileBrokerServiceAccount(ctx, ns, resources.MakeServiceAccount(ns, saName))
	if err != nil {
		return fmt.Errorf("service account '%s': %w", saName, err)
	}

	_, err = r.reconcileBrokerRBAC(ctx, ns, sa, resources.MakeRoleBinding(rbName, ns, ns.Name, sa, clusterRoleName))
	if err != nil {
		return fmt.Errorf("role binding '%s': %w", rbName, err)
	}

	// If the Broker pull secret has not been specified, then nothing to copy.
	if r.brokerPullSecretName == "" {
		return nil
	}

	if sa.Name == resources.IngressServiceAccountName || sa.Name == resources.FilterServiceAccountName {
		// check for existence of brokerPullSecret, and skip copy if it already exists
		for _, v := range sa.ImagePullSecrets {
			if fmt.Sprintf("%s", v) == ("{" + r.brokerPullSecretName + "}") {
				return nil
			}
		}
		_, err := utils.CopySecret(r.KubeClientSet.CoreV1(), system.Namespace(), r.brokerPullSecretName, ns.Name, sa.Name)
		if err != nil {
			r.Recorder.Event(ns, corev1.EventTypeWarning, secretCopyFailure,
				fmt.Sprintf("Error copying secret %s/%s => %s/%s : %v", system.Namespace(), r.brokerPullSecretName, ns.Name, sa.Name, err))
			return fmt.Errorf("Error copying secret %s/%s => %s/%s : %w", system.Namespace(), r.brokerPullSecretName, ns.Name, sa.Name, err)
		} else {
			r.Recorder.Event(ns, corev1.EventTypeNormal, secretCopied,
				fmt.Sprintf("Secret copied into namespace %s/%s => %s/%s", system.Namespace(), r.brokerPullSecretName, ns.Name, sa.Name))
		}
	}

	return nil
}

// reconcileBrokerServiceAccount reconciles the Broker's service account for Namespace 'ns'.
func (r *Reconciler) reconcileBrokerServiceAccount(ctx context.Context, ns *corev1.Namespace, sa *corev1.ServiceAccount) (*corev1.ServiceAccount, error) {
	current, err := r.serviceAccountLister.ServiceAccounts(ns.Name).Get(sa.Name)

	// If the resource doesn't exist, we'll create it.
	if k8serrors.IsNotFound(err) {
		sa, err = r.KubeClientSet.CoreV1().ServiceAccounts(ns.Name).Create(sa)
		if err != nil {
			return nil, err
		}
		r.Recorder.Event(ns, corev1.EventTypeNormal, serviceAccountCreated,
			fmt.Sprintf("ServiceAccount '%s' created for the Broker", sa.Name))
		return sa, nil
	} else if err != nil {
		return nil, err
	}
	// Don't update anything that is already present.
	return current, nil
}

// reconcileBrokerRBAC reconciles the Broker's service account RBAC for the Namespace 'ns'.
func (r *Reconciler) reconcileBrokerRBAC(ctx context.Context, ns *corev1.Namespace, sa *corev1.ServiceAccount, rb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
	current, err := r.roleBindingLister.RoleBindings(rb.Namespace).Get(rb.Name)

	// If the resource doesn't exist, we'll create it.
	if k8serrors.IsNotFound(err) {
		rb, err = r.KubeClientSet.RbacV1().RoleBindings(rb.Namespace).Create(rb)
		if err != nil {
			return nil, err
		}
		r.Recorder.Event(ns, corev1.EventTypeNormal, serviceAccountRBACCreated,
			fmt.Sprintf("RoleBinding '%s/%s' created for the Broker", rb.Namespace, rb.Name))
		return rb, nil
	} else if err != nil {
		return nil, err
	}
	// Don't update anything that is already present.
	return current, nil
}

// reconcileBroker reconciles the default Broker for the Namespace 'ns'.
func (r *Reconciler) reconcileBroker(ctx context.Context, ns *corev1.Namespace) (*v1beta1.Broker, error) {
	current, err := r.brokerLister.Brokers(ns.Name).Get(resources.DefaultBrokerName)

	// If the resource doesn't exist, we'll create it.
	if k8serrors.IsNotFound(err) {
		b := resources.MakeBroker(ns)
		b, err = r.EventingClientSet.EventingV1beta1().Brokers(ns.Name).Create(b)
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
