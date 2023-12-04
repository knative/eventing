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
	"knative.dev/eventing/pkg/apis/feature"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgreconciler "knative.dev/pkg/reconciler"
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

type OIDCIdentityStatusMarker interface {
	MarkOIDCIdentityCreatedSucceeded()
	MarkOIDCIdentityCreatedSucceededWithReason(reason, messageFormat string, messageA ...interface{})
	MarkOIDCIdentityCreatedFailed(reason, messageFormat string, messageA ...interface{})
}

func SetupOIDCServiceAccount(ctx context.Context, flags feature.Flags, serviceAccountLister corev1listers.ServiceAccountLister, kubeclient kubernetes.Interface, gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta, marker OIDCIdentityStatusMarker, setAuthStatus func(a *duckv1.AuthStatus)) pkgreconciler.Event {
	if flags.IsOIDCAuthentication() {
		saName := GetOIDCServiceAccountNameForResource(gvk, objectMeta)
		setAuthStatus(&duckv1.AuthStatus{
			ServiceAccountName: &saName,
		})
		if err := EnsureOIDCServiceAccountExistsForResource(ctx, serviceAccountLister, kubeclient, gvk, objectMeta); err != nil {
			marker.MarkOIDCIdentityCreatedFailed("Unable to resolve service account for OIDC authentication", "%v", err)
			return err
		}
		marker.MarkOIDCIdentityCreatedSucceeded()
	} else {
		setAuthStatus(nil)
		marker.MarkOIDCIdentityCreatedSucceededWithReason(fmt.Sprintf("%s feature disabled", feature.OIDCAuthentication), "")
	}
	return nil
}

// EnsureOIDCServiceAccountRoleBindingExistsForResource
// makes sure the given resource has an OIDC service account role binding with
// an owner reference to the resource set.
func EnsureOIDCServiceAccountRoleBindingExistsForResource(ctx context.Context, kubeclient kubernetes.Interface, gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta, saName string) error {
	logger := logging.FromContext(ctx)
	logger.Errorf("haha: Initializing")

	roleName := fmt.Sprintf("create-oidc-token")
	roleBindingName := fmt.Sprintf("create-oidc-token")

	logger.Errorf("haha: going to get role binding for %s", objectMeta.Name)
	logger.Errorf("haha: role name %s", roleName)
	logger.Errorf("haha: role binding name %s", roleBindingName)

	roleBinding, err := kubeclient.RbacV1().RoleBindings(objectMeta.Namespace).Get(ctx, roleBindingName, metav1.GetOptions{})

	logger.Errorf("haha: got role binding for %s", roleBinding)

	logger.Errorf("haha: going to enter the if statement")
	// If the resource doesn't exist, we'll create it.
	if apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Debugw("Creating OIDC service account role binding", zap.Error(err))
		logger.Errorf("haha: creating role binding")

		// Create the "create-oidc-token" role
		CreateRoleForServiceAccount(ctx, kubeclient, gvk, objectMeta)

		roleBinding = &rbacv1.RoleBinding{
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
				//objectMeta.Name + roleName
				Name: fmt.Sprintf(roleName),
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Namespace: objectMeta.GetNamespace(),
					// apiServerSource service account name, it is in the source.Spec, NOT in source.Auth
					Name: saName,
				},
			},
		}

		logger.Errorf("haha: role binding object created")
		logger.Errorf("haha: role binding object %s", roleBinding)

		_, err = kubeclient.RbacV1().RoleBindings(objectMeta.Namespace).Create(ctx, roleBinding, metav1.CreateOptions{})
		if err != nil {
			logger.Errorf("haha: error creating role binding")
			logger.Errorf("haha: error %s", err)
			return fmt.Errorf("could not create OIDC service account role binding %s/%s for %s: %w", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)
		}

		return nil
	}

	if err != nil {
		logger.Errorf("haha: error getting role binding")
		logger.Errorf("haha: error %s", err)
		return fmt.Errorf("could not get OIDC service account role binding %s/%s for %s: %w", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)

	}

	if !metav1.IsControlledBy(&roleBinding.ObjectMeta, &objectMeta) {
		logger.Errorf("haha: role binding not owned by")
		logger.Errorf("haha: role binding %s", roleBinding)
		return fmt.Errorf("role binding %s not owned by %s %s", roleBinding.Name, gvk.Kind, objectMeta.Name)
	}

	return nil
}

// Create the create-oidc-token role for the service account
func CreateRoleForServiceAccount(ctx context.Context, kubeclient kubernetes.Interface, gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta) error {

	logger := logging.FromContext(ctx)
	logger.Errorf("haha: Initializing create role for service account")

	roleName := fmt.Sprintf("create-oidc-token")
	logger.Errorf("haha: role name %s", roleName)

	role, err := kubeclient.RbacV1().Roles(objectMeta.Namespace).Get(ctx, roleName, metav1.GetOptions{})
	logger.Errorf("haha: got role %s", role)

	// If the resource doesn't exist, we'll create it.
	if apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Debugw("Creating OIDC service account role", zap.Error(err))

		logger.Errorf("haha: creating role")

		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleName,
				Namespace: objectMeta.GetNamespace(),
				Annotations: map[string]string{
					"description": fmt.Sprintf("Role for OIDC Authentication for %s %q", gvk.GroupKind().Kind, objectMeta.Name),
				},
			},
			Rules: []rbacv1.PolicyRule{
				rbacv1.PolicyRule{
					APIGroups: []string{""},

					// serviceaccount name
					ResourceNames: []string{GetOIDCServiceAccountNameForResource(gvk, objectMeta)},
					Resources:     []string{"serviceaccounts/token"},
					Verbs:         []string{"create"},
				},
			},
		}

		logger.Errorf("haha: role object created")
		logger.Errorf("haha: role object %s", role)

		_, err = kubeclient.RbacV1().Roles(objectMeta.Namespace).Create(ctx, role, metav1.CreateOptions{})
		if err != nil {
			logger.Errorf("haha: error creating role")
			logger.Errorf("haha: error %s", err)
			return fmt.Errorf("could not create OIDC service account role %s/%s for %s: %w", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)
		}

		return nil
	}

	if err != nil {
		logger.Errorf("haha: error getting role")
		logger.Errorf("haha: error getting role %s", err)
		return fmt.Errorf("could not get OIDC service account role %s/%s for %s: %w", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)

	}
	return nil
}
