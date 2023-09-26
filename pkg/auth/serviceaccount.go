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
	"strings"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
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

func EnsureOIDCServiceAccountExistsForResource(ctx context.Context, kubeclient kubernetes.Interface, gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta) error {
	saName := GetOIDCServiceAccountNameForResource(gvk, objectMeta)
	sa, err := kubeclient.CoreV1().ServiceAccounts(objectMeta.Namespace).Get(ctx, saName, metav1.GetOptions{})

	// If the resource doesn't exist, we'll create it.
	if apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Infow("Creating OIDC service account", zap.Error(err))

		expected := GetOIDCServiceAccountForResource(gvk, objectMeta)

		_, err = kubeclient.CoreV1().ServiceAccounts(objectMeta.Namespace).Create(ctx, expected, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("could not create OIDC service account %s/%s for %s: %w", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)
		}
		return nil
	} else if err != nil {
		logging.FromContext(ctx).Errorw("Failed to get OIDC service account", zap.Error(err))

		return fmt.Errorf("could not get OIDC service account %s/%s for %s: %w", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)
	} else if !metav1.IsControlledBy(&sa.ObjectMeta, &objectMeta) {
		logging.FromContext(ctx).Errorw("Service account not owned by parent resource", zap.Any("service account", sa), zap.Any("parent resource ("+gvk.Kind+")", objectMeta))

		return fmt.Errorf("%s %q does not own service account %q", gvk.Kind, objectMeta.Name, sa.Name)
	}

	return nil
}
