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
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/resolver"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	applyconfigurationcorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applyconfigurationmetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/utils/pointer"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracker"
	"knative.dev/pkg/webhook/psbinding"

	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/eventingtls"

	"knative.dev/eventing/pkg/apis/feature"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
)

type SinkBindingSubResourcesReconciler struct {
	res                        *resolver.URIResolver
	tracker                    tracker.Interface
	serviceAccountLister       corev1listers.ServiceAccountLister
	secretLister               corev1listers.SecretLister
	kubeclient                 kubernetes.Interface
	featureStore               *feature.Store
	tokenProvider              *auth.OIDCTokenProvider
	trustBundleConfigMapLister corev1listers.ConfigMapLister
}

func (s *SinkBindingSubResourcesReconciler) Reconcile(ctx context.Context, b psbinding.Bindable) error {
	sb := b.(*v1.SinkBinding)
	if s.res == nil {
		err := errors.New("Resolver is nil")
		logging.FromContext(ctx).Errorf("%w", err)
		sb.Status.MarkBindingUnavailable("NoResolver", "No Resolver associated with context for sink")
		return err
	}

	if err := s.propagateTrustBundles(ctx, sb); err != nil {
		sb.Status.MarkBindingUnavailable("TrustBundlePropagation", err.Error())
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

		if err := s.removeOIDCTokenSecretEventually(ctx, sb); err != nil {
			return err
		}
		sb.Status.OIDCTokenSecretName = nil
	}

	return nil
}

// I'm just here so I won't get fined
func (*SinkBindingSubResourcesReconciler) ReconcileDeletion(ctx context.Context, b psbinding.Bindable) error {
	return nil
}

func (s *SinkBindingSubResourcesReconciler) reconcileOIDCTokenSecret(ctx context.Context, sb *v1.SinkBinding) error {
	logger := logging.FromContext(ctx)
	secretName := s.oidcTokenSecretName(sb)

	if sb.Status.SinkAudience == nil {
		return fmt.Errorf("sinkAudience must be set on %s/%s to generate a OIDC token secret", sb.Name, sb.Namespace)
	}

	secret, err := s.secretLister.Secrets(sb.Namespace).Get(secretName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			// create new secret
			logger.Debugf("No OIDC token secret found for %s/%s sinkbinding. Will create a new secret", sb.Name, sb.Namespace)

			return s.renewOIDCTokenSecret(ctx, sb)
		}

		return fmt.Errorf("could not check if secret %q exists already: %w", secretName, err)
	}

	// check if token needs to be renewed
	expiry, err := auth.GetJWTExpiry(string(secret.Data["token"]))
	if err != nil {
		logger.Warnf("Could not get expiry date of OIDC token secret: %s. Will renew token.", err)

		return s.renewOIDCTokenSecret(ctx, sb)
	}

	resyncAndBufferDuration := resyncPeriod + tokenExpiryBuffer
	if expiry.After(time.Now().Add(resyncAndBufferDuration)) {
		logger.Debugf("OIDC token secret for %s/%s sinkbinding still valid for > %s (expires %s). Will not update secret", sb.Name, sb.Namespace, resyncAndBufferDuration, expiry)
		// token is still valid for resync period + buffer --> we're fine

		return nil
	}

	logger.Debugf("OIDC token secret for %s/%s sinkbinding is valid for less than %s (expires %s). Will update secret", sb.Name, sb.Namespace, resyncAndBufferDuration, expiry)

	return s.renewOIDCTokenSecret(ctx, sb)
}

func (s *SinkBindingSubResourcesReconciler) renewOIDCTokenSecret(ctx context.Context, sb *v1.SinkBinding) error {
	logger := logging.FromContext(ctx)
	secretName := s.oidcTokenSecretName(sb)

	token, err := s.tokenProvider.GetNewJWT(types.NamespacedName{
		Namespace: sb.Namespace,
		Name:      *sb.Status.Auth.ServiceAccountName,
	}, *sb.Status.SinkAudience)

	if err != nil {
		return fmt.Errorf("could not create token for SinkBinding %s/%s: %w", sb.Name, sb.Namespace, err)
	}

	apiVersion := fmt.Sprintf("%s/%s", v1.SchemeGroupVersion.Group, v1.SchemeGroupVersion.Version)
	applyConfig := new(applyconfigurationcorev1.SecretApplyConfiguration).
		WithName(secretName).
		WithNamespace(sb.Namespace).
		WithType(corev1.SecretTypeOpaque).
		WithKind("Secret").
		WithAPIVersion("v1").
		WithOwnerReferences(&applyconfigurationmetav1.OwnerReferenceApplyConfiguration{
			APIVersion:         &apiVersion,
			Kind:               pointer.String("SinkBinding"),
			Name:               &sb.Name,
			UID:                &sb.UID,
			Controller:         pointer.Bool(true),
			BlockOwnerDeletion: pointer.Bool(false),
		}).
		WithStringData(map[string]string{
			"token": token,
		})

	_, err = s.kubeclient.CoreV1().Secrets(sb.Namespace).Apply(ctx, applyConfig, metav1.ApplyOptions{FieldManager: controllerAgentName})
	if err != nil {
		return fmt.Errorf("could not create or update OIDC token secret for SinkBinding %s/%s: %w", sb.Name, sb.Namespace, err)
	}

	logger.Debugf("Created/Updated OIDC token secret for %s/%s sinkbinding with new token.", sb.Name, sb.Namespace)

	sb.Status.OIDCTokenSecretName = &secretName

	return nil
}

func (s *SinkBindingSubResourcesReconciler) oidcTokenSecretName(sb *v1.SinkBinding) string {
	return kmeta.ChildName(sb.Name, "-oidc-token")
}

func (s *SinkBindingSubResourcesReconciler) removeOIDCTokenSecretEventually(ctx context.Context, sb *v1.SinkBinding) error {
	if sb.Status.OIDCTokenSecretName == nil {
		return nil
	}

	_, err := s.secretLister.Secrets(sb.Namespace).Get(*sb.Status.OIDCTokenSecretName)
	if apierrs.IsNotFound(err) {
		return nil
	}

	return s.kubeclient.CoreV1().Secrets(sb.Namespace).Delete(ctx, *sb.Status.OIDCTokenSecretName, metav1.DeleteOptions{})
}

func (s *SinkBindingSubResourcesReconciler) propagateTrustBundles(ctx context.Context, sb *v1.SinkBinding) error {
	gvk := schema.GroupVersionKind{
		Group:   v1.SchemeGroupVersion.Group,
		Version: v1.SchemeGroupVersion.Version,
		Kind:    "SinkBinding",
	}
	return eventingtls.PropagateTrustBundles(ctx, s.kubeclient, s.trustBundleConfigMapLister, gvk, sb)
}
