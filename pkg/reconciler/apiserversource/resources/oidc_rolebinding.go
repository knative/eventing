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

package resources

import (
	"fmt"

	"knative.dev/eventing/pkg/apis/sources"

	"knative.dev/pkg/kmeta"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
)

// GetOIDCTokenRoleName will return the name of the role for creating the JWT token
func GetOIDCTokenRoleName(sourceName string) string {
	return kmeta.ChildName(sourceName, "-create-oidc-token")
}

// GetOIDCTokenRoleBindingName will return the name of the rolebinding for creating the JWT token
func GetOIDCTokenRoleBindingName(sourceName string) string {
	return kmeta.ChildName(sourceName, "-create-oidc-token")
}

// MakeOIDCRole will return the role object config for generating the JWT token
func MakeOIDCRole(source *v1.ApiServerSource) (*rbacv1.Role, error) {
	roleName := GetOIDCTokenRoleName(source.Name)

	if source.Status.Auth == nil || source.Status.Auth.ServiceAccountName == nil {
		return nil, fmt.Errorf("Error when making OIDC Role for apiserversource, as the OIDC service account does not exist")
	}

	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: source.GetNamespace(),
			Annotations: map[string]string{
				"description": fmt.Sprintf("Role for OIDC Authentication for ApiServerSource %q", source.GetName()),
			},
			Labels: map[string]string{
				sources.OIDCLabelKey: "",
			},
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(source),
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				// apiServerSource OIDC service account name, it is in the source.Status, NOT in source.Spec
				ResourceNames: []string{*source.Status.Auth.ServiceAccountName},
				Resources:     []string{"serviceaccounts/token"},
				Verbs:         []string{"create"},
			},
		},
	}, nil

}

// MakeOIDCRoleBinding will return the rolebinding object for generating the JWT token
// So that ApiServerSource's service account have access to create the JWT token for it's OIDC service account and the target audience
// Note:  it is in the source.Spec, NOT in source.Auth
func MakeOIDCRoleBinding(source *v1.ApiServerSource) (*rbacv1.RoleBinding, error) {
	roleName := GetOIDCTokenRoleName(source.Name)
	roleBindingName := GetOIDCTokenRoleBindingName(source.Name)

	if source.Spec.ServiceAccountName == "" {
		return nil, fmt.Errorf("Error when making OIDC RoleBinding for apiserversource, as the Spec service account does not exist")
	}

	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindingName,
			Namespace: source.GetNamespace(),
			Annotations: map[string]string{
				"description": fmt.Sprintf("Role Binding for OIDC Authentication for ApiServerSource %q", source.GetName()),
			},
			Labels: map[string]string{
				sources.OIDCLabelKey: "",
			},
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(source),
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
				Namespace: source.GetNamespace(),
				//Note: apiServerSource service account name, it is in the source.Spec, NOT in source.Status.Auth
				Name: source.Spec.ServiceAccountName,
			},
		},
	}, nil

}
