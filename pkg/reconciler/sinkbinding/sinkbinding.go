/*
Copyright 2020 The Knative Authors

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

package sinkbinding

import (
	"context"
	"errors"
	"fmt"

	"knative.dev/eventing/pkg/auth"
	"knative.dev/pkg/resolver"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/eventing/pkg/apis/feature"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracker"
	"knative.dev/pkg/webhook/psbinding"
)

type SinkBindingSubResourcesReconciler struct {
	res                  *resolver.URIResolver
	tracker              tracker.Interface
	serviceAccountLister corev1listers.ServiceAccountLister
	kubeclient           kubernetes.Interface
	featureStore         *feature.Store
	tokenProvider        *auth.OIDCTokenProvider
}

func (s *SinkBindingSubResourcesReconciler) Reconcile(ctx context.Context, b psbinding.Bindable) error {
	sb := b.(*v1.SinkBinding)
	if s.res == nil {
		err := errors.New("Resolver is nil")
		logging.FromContext(ctx).Errorf("%w", err)
		sb.Status.MarkBindingUnavailable("NoResolver", "No Resolver associated with context for sink")
		return err
	}
	if sb.Spec.Sink.Ref != nil {
		s.tracker.TrackReference(tracker.Reference{
			APIVersion: sb.Spec.Sink.Ref.APIVersion,
			Kind:       sb.Spec.Sink.Ref.Kind,
			Namespace:  sb.Spec.Sink.Ref.Namespace,
			Name:       sb.Spec.Sink.Ref.Name,
		}, b)
	}

	addr, err := s.res.AddressableFromDestinationV1(ctx, sb.Spec.Sink, sb)
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to get Addressable from Destination: %w", err)
		sb.Status.MarkBindingUnavailable("NoAddressable", "Addressable could not be extracted from destination")
		return err
	}
	sb.Status.MarkSink(addr)

	featureFlags := s.featureStore.Load()
	if featureFlags.IsOIDCAuthentication() {
		saName := auth.GetOIDCServiceAccountNameForResource(v1.SchemeGroupVersion.WithKind("SinkBinding"), sb.ObjectMeta)
		sb.Status.Auth = &duckv1.AuthStatus{
			ServiceAccountName: &saName,
		}

		if err := auth.EnsureOIDCServiceAccountExistsForResource(ctx, s.serviceAccountLister, s.kubeclient, v1.SchemeGroupVersion.WithKind("SinkBinding"), sb.ObjectMeta); err != nil {
			sb.Status.MarkOIDCIdentityCreatedFailed("Unable to resolve service account for OIDC authentication", "%v", err)
			return err
		}
		sb.Status.MarkOIDCIdentityCreatedSucceeded()

		err := s.reconcileOIDCTokenSecret(ctx, sb)
		if err != nil {
			sb.Status.MarkOIDCTokenSecretCreatedFailed("Unable to reconcile OIDC token secret", "%v", err)
			return err
		}
		sb.Status.MarkOIDCTokenSecretCreatedSuccceeded()

	} else {
		sb.Status.Auth = nil
		sb.Status.MarkOIDCIdentityCreatedSucceededWithReason(fmt.Sprintf("%s feature disabled", feature.OIDCAuthentication), "")
		sb.Status.MarkOIDCTokenSecretCreatedSuccceededWithReason(fmt.Sprintf("%s feature disabled", feature.OIDCAuthentication), "")
		sb.Status.OIDCTokenSecretName = nil
	}

	return nil
}

// I'm just here so I won't get fined
func (*SinkBindingSubResourcesReconciler) ReconcileDeletion(ctx context.Context, b psbinding.Bindable) error {
	return nil
}

func (s *SinkBindingSubResourcesReconciler) reconcileOIDCTokenSecret(ctx context.Context, sb *v1.SinkBinding) error {
	secretName := fmt.Sprintf("oidc-token-%s", sb.Name)

	if sb.Status.SinkAudience == nil {
		return fmt.Errorf("sinkAudience must be set on %s/%s to generate a OIDC token secret", sb.Name, sb.Namespace)
	}

	token, err := s.tokenProvider.GetJWT(types.NamespacedName{
		Namespace: sb.Namespace,
		Name:      *sb.Status.Auth.ServiceAccountName,
	}, *sb.Status.SinkAudience)

	if err != nil {
		return fmt.Errorf("could not create token for SinkBinding %s/%s: %w", sb.Name, sb.Namespace, err)
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: sb.Namespace,
		},
		StringData: map[string]string{
			"token": token,
		},
		Type: corev1.SecretTypeOpaque,
	}

	_, err = s.kubeclient.CoreV1().Secrets(sb.Namespace).Create(ctx, &secret, metav1.CreateOptions{})
	if err != nil && !apierrs.IsAlreadyExists(err) {
		return fmt.Errorf("could not create OIDC token secret for SinkBinding %s/%s: %w", sb.Name, sb.Namespace, err)
	}

	sb.Status.OIDCTokenSecretName = &secretName

	return nil
}
