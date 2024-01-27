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

	"knative.dev/eventing/pkg/apis/feature"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
	pkgreconciler "knative.dev/pkg/reconciler"

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

const (
	//OIDCLabelKey is used to filter out all the informers that related to OIDC work
	OIDCLabelKey = "oidc"

	// OIDCTokenRoleLabelSelector is the label selector for the OIDC token creator role and rolebinding informers
	OIDCLabelSelector = OIDCLabelKey
)

// GetOIDCServiceAccountNameForResource returns the service account name to use
// for OIDC authentication for the given resource.
func GetOIDCServiceAccountNameForResource(gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta) string {
	suffix := fmt.Sprintf("-oidc-%s-%s", gvk.Group, gvk.Kind)
	parent := objectMeta.GetName()
	sa := kmeta.ChildName(parent, suffix)
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
			Labels: map[string]string{
				OIDCLabelKey: "enabled",
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

// DeleteOIDCServiceAccountIfExists makes sure the given resource does not have an OIDC service account.
// If it does that service account is deleted.
func DeleteOIDCServiceAccountIfExists(ctx context.Context, serviceAccountLister corev1listers.ServiceAccountLister, kubeclient kubernetes.Interface, gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta) error {
	saName := GetOIDCServiceAccountNameForResource(gvk, objectMeta)
	sa, err := serviceAccountLister.ServiceAccounts(objectMeta.Namespace).Get(saName)

	if err == nil && metav1.IsControlledBy(&sa.ObjectMeta, &objectMeta) {
		logging.FromContext(ctx).Debugf("OIDC Service account exists and has correct owner (%s/%s). Deleting OIDC service account", objectMeta.Name, objectMeta.Namespace)

		err = kubeclient.CoreV1().ServiceAccounts(objectMeta.Namespace).Delete(ctx, sa.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("could not delete OIDC service account %s/%s for %s: %w", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)
		}
	} else if apierrs.IsNotFound(err) {
		return nil
	}

	return err
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
		if err := DeleteOIDCServiceAccountIfExists(ctx, serviceAccountLister, kubeclient, gvk, objectMeta); err != nil {
			return err
		}
		setAuthStatus(nil)
		marker.MarkOIDCIdentityCreatedSucceededWithReason(fmt.Sprintf("%s feature disabled", feature.OIDCAuthentication), "")
	}
	return nil
}
