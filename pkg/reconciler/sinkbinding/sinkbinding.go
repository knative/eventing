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
	} else {
		sb.Status.Auth = nil
		sb.Status.MarkOIDCIdentityCreatedSucceededWithReason(fmt.Sprintf("%s feature disabled", feature.OIDCAuthentication), "")
	}

	addr, err := s.res.AddressableFromDestinationV1(ctx, sb.Spec.Sink, sb)
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to get Addressable from Destination: %w", err)
		sb.Status.MarkBindingUnavailable("NoAddressable", "Addressable could not be extracted from destination")
		return err
	}
	sb.Status.MarkSink(addr)
	return nil
}

// I'm just here so I won't get fined
func (*SinkBindingSubResourcesReconciler) ReconcileDeletion(ctx context.Context, b psbinding.Bindable) error {
	return nil
}
