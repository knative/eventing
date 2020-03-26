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

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"

	configsv1alpha1 "knative.dev/eventing/pkg/apis/configs/v1alpha1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	configslisters "knative.dev/eventing/pkg/client/listers/configs/v1alpha1"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1beta1"
	"knative.dev/eventing/pkg/reconciler/namespace/resources"
	"knative.dev/eventing/pkg/utils"
	namespacereconciler "knative.dev/pkg/client/injection/kube/reconciler/core/v1/namespace"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process.
	configMapPropagationCreated = "ConfigMapPropagationCreated"
	brokerCreated               = "BrokerCreated"
	serviceAccountCreated       = "BrokerServiceAccountCreated"
	serviceAccountRBACCreated   = "BrokerServiceAccountRBACCreated"
	secretCopied                = "SecretCopied"
	secretCopyFailure           = "SecretCopyFailure"
)

type Reconciler struct {
	brokerPullSecretName string

	eventingClientSet clientset.Interface
	kubeClientSet     kubernetes.Interface

	// listers index properties about resources
	namespaceLister            corev1listers.NamespaceLister
	serviceAccountLister       corev1listers.ServiceAccountLister
	roleBindingLister          rbacv1listers.RoleBindingLister
	brokerLister               eventinglisters.BrokerLister
	configMapPropagationLister configslisters.ConfigMapPropagationLister
}

// Check that our Reconciler implements namespacereconciler.Interface
var _ namespacereconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, ns *corev1.Namespace) reconciler.Event {
	if ns.Labels[resources.InjectionLabelKey] != resources.InjectionEnabledLabelValue {
		logging.FromContext(ctx).Debug("Not reconciling Namespace")
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
		cmp, err = r.eventingClientSet.ConfigsV1alpha1().ConfigMapPropagations(ns.Name).Create(cmp)
		if err != nil {
			return nil, err
		}
		controller.GetEventRecorder(ctx).Event(ns, corev1.EventTypeNormal, configMapPropagationCreated,
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
		_, err := utils.CopySecret(r.kubeClientSet.CoreV1(), system.Namespace(), r.brokerPullSecretName, ns.Name, sa.Name)
		if err != nil {
			controller.GetEventRecorder(ctx).Event(ns, corev1.EventTypeWarning, secretCopyFailure,
				fmt.Sprintf("Error copying secret %s/%s => %s/%s : %v", system.Namespace(), r.brokerPullSecretName, ns.Name, sa.Name, err))
			return fmt.Errorf("Error copying secret %s/%s => %s/%s : %w", system.Namespace(), r.brokerPullSecretName, ns.Name, sa.Name, err)
		} else {
			controller.GetEventRecorder(ctx).Event(ns, corev1.EventTypeNormal, secretCopied,
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
		sa, err = r.kubeClientSet.CoreV1().ServiceAccounts(ns.Name).Create(sa)
		if err != nil {
			return nil, err
		}
		controller.GetEventRecorder(ctx).Event(ns, corev1.EventTypeNormal, serviceAccountCreated,
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
		rb, err = r.kubeClientSet.RbacV1().RoleBindings(rb.Namespace).Create(rb)
		if err != nil {
			return nil, err
		}
		controller.GetEventRecorder(ctx).Event(ns, corev1.EventTypeNormal, serviceAccountRBACCreated,
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
		b, err = r.eventingClientSet.EventingV1beta1().Brokers(ns.Name).Create(b)
		if err != nil {
			return nil, err
		}
		controller.GetEventRecorder(ctx).Event(ns, corev1.EventTypeNormal, brokerCreated,
			"Default eventing.knative.dev Broker created.")
		return b, nil
	} else if err != nil {
		return nil, err
	}
	// Don't update anything that is already present.
	return current, nil
}
