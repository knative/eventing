/*
Copyright 2023 The Knative Authors

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

package auth

import (
	"context"
	"fmt"
	rbacv1 "k8s.io/api/rbac/v1"
	"strings"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
)

// GetOIDCServiceAccountNameForResource returns the service account name to use
// for OIDC authentication for the given resource.
func GetOIDCServiceAccountNameForResource(gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta) string {
	sa := fmt.Sprintf("oidc-%s-%s-%s", gvk.GroupKind().Group, gvk.GroupKind().Kind, objectMeta.GetName())

	return strings.ToLower(sa)
}

// GetOIDCServiceAccountForResource returns the service account to use for OIDC
// authentication for the given resource.
func GetOIDCServiceAccountForResource(gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetOIDCServiceAccountNameForResource(gvk, objectMeta),
			Namespace: objectMeta.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         gvk.GroupKind().Group + "/" + gvk.GroupVersion().Version,
					Kind:               gvk.GroupKind().Kind,
					Name:               objectMeta.GetName(),
					UID:                objectMeta.GetUID(),
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(false),
				},
			},
			Annotations: map[string]string{
				"description": fmt.Sprintf("Service Account for OIDC Authentication for %s %q", gvk.GroupKind().Kind, objectMeta.Name),
			},
		},
	}
}

// EnsureOIDCServiceAccountExistsForResource makes sure the given resource has
// an OIDC service account with an owner reference to the resource set.
func EnsureOIDCServiceAccountExistsForResource(ctx context.Context, serviceAccountLister corev1listers.ServiceAccountLister, kubeclient kubernetes.Interface, gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta) error {
	saName := GetOIDCServiceAccountNameForResource(gvk, objectMeta)
	sa, err := serviceAccountLister.ServiceAccounts(objectMeta.Namespace).Get(saName)

	// If the resource doesn't exist, we'll create it.
	if apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Debugw("Creating OIDC service account", zap.Error(err))

		expected := GetOIDCServiceAccountForResource(gvk, objectMeta)

		_, err = kubeclient.CoreV1().ServiceAccounts(objectMeta.Namespace).Create(ctx, expected, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("could not create OIDC service account %s/%s for %s: %w", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)
		}

		return nil
	}

	if err != nil {
		return fmt.Errorf("could not get OIDC service account %s/%s for %s: %w", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)
	}

	if !metav1.IsControlledBy(&sa.ObjectMeta, &objectMeta) {
		return fmt.Errorf("service account %s not owned by %s %s", sa.Name, gvk.Kind, objectMeta.Name)
	}

	return nil
}

// EnsureOIDCServiceAccountRoleBindingExistsForResource
// makes sure the given resource has an OIDC service account role binding with
// an owner reference to the resource set.
func EnsureOIDCServiceAccountRoleBindingExistsForResource(ctx context.Context, kubeclient kubernetes.Interface, gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta) error {
	roleName := fmt.Sprintf("create-oidc-token")
	roleBindingName := fmt.Sprintf("create-oidc-token", objectMeta.GetName())
	roleBinding, err := kubeclient.RbacV1().RoleBindings(objectMeta.Namespace).Get(ctx, roleBindingName, metav1.GetOptions{})

	// If the resource doesn't exist, we'll create it.
	if apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Debugw("Creating OIDC service account role binding", zap.Error(err))

		// Create the "create-oidc-token" role
		CreateRoleForServiceAccount(ctx, kubeclient, gvk, objectMeta)

		roleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleBindingName,
				Namespace: objectMeta.GetNamespace(),
				Annotations: map[string]string{
					"description": fmt.Sprintf("Role Binding for OIDC Authentication for %s %q", gvk.GroupKind().Kind, objectMeta.Name),
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     roleName,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Namespace: objectMeta.GetNamespace(),
					Name:      GetOIDCServiceAccountNameForResource(gvk, objectMeta),
				},
			},
		}

		_, err = kubeclient.RbacV1().RoleBindings(objectMeta.Namespace).Create(ctx, roleBinding, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("could not create OIDC service account role binding %s/%s for %s: %w", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)
		}

		return nil
	}

	if err != nil {
		return fmt.Errorf("could not get OIDC service account role binding %s/%s for %s: %w", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)

	}

	if !metav1.IsControlledBy(&roleBinding.ObjectMeta, &objectMeta) {
		return fmt.Errorf("role binding %s not owned by %s %s", roleBinding.Name, gvk.Kind, objectMeta.Name)
	}

	return nil
}

// Create the create-oidc-token role for the service account
func CreateRoleForServiceAccount(ctx context.Context, kubeclient kubernetes.Interface, gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta) error {
	roleName := fmt.Sprintf("create-oidc-token")
	role, err := kubeclient.RbacV1().Roles(objectMeta.Namespace).Get(ctx, roleName, metav1.GetOptions{})

	// If the resource doesn't exist, we'll create it.
	if apierrs.IsNotFound(err) && role == nil {
		logging.FromContext(ctx).Debugw("Creating OIDC service account role", zap.Error(err))

		role := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleName,
				Namespace: objectMeta.GetNamespace(),
				Annotations: map[string]string{
					"description": fmt.Sprintf("Role for OIDC Authentication for %s %q", gvk.GroupKind().Kind, objectMeta.Name),
				},
			},
			Rules: []rbacv1.PolicyRule{
				rbacv1.PolicyRule{
					APIGroups:     []string{""},
					ResourceNames: []string{objectMeta.Name},
					Resources:     []string{"serviceaccounts/token"},
					Verbs:         []string{"create"},
				},
			},
		}

		_, err = kubeclient.RbacV1().Roles(objectMeta.Namespace).Create(ctx, role, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("could not create OIDC service account role %s/%s for %s: %w", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)
		}

		return nil
	}

	if err != nil {
		return fmt.Errorf("could not get OIDC service account role %s/%s for %s: %w", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)

	}
	return nil
}
