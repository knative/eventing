/*
Copyright 2024 The Knative Authors

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

package v1alpha1

import (
	"context"

	"knative.dev/eventing/pkg/apis/feature"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/apis/sinks/v1alpha1"
)

// IntegrationSinkOption enables further configuration of a IntegrationSink.
type IntegrationSinkOption func(source *v1alpha1.IntegrationSink)

// NewIntegrationSink creates a v1 IntegrationSink with IntegrationSinkOptions
func NewIntegrationSink(name, namespace string, o ...IntegrationSinkOption) *v1alpha1.IntegrationSink {
	s := &v1alpha1.IntegrationSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range o {
		opt(s)
	}
	s.SetDefaults(context.Background())
	return s
}

func WithIntegrationSinkUID(uid types.UID) IntegrationSinkOption {
	return func(s *v1alpha1.IntegrationSink) {
		s.UID = uid
	}
}

// WithInitIntegrationSinkConditions initializes the IntegrationSink's conditions.
func WithInitIntegrationSinkConditions(s *v1alpha1.IntegrationSink) {
	s.Status.InitializeConditions()
}

func WithIntegrationSinkPropagateDeploymenteStatus(deployment *appsv1.Deployment) IntegrationSinkOption {
	return func(s *v1alpha1.IntegrationSink) {
		s.Status.PropagateDeploymentStatus(deployment)
	}
}

func WithIntegrationSinkAddressableReady() IntegrationSinkOption {
	return func(s *v1alpha1.IntegrationSink) {
		s.Status.MarkAddressableReady()
	}
}

func WithIntegrationSinkAddress(addr duckv1.Addressable) IntegrationSinkOption {
	return func(s *v1alpha1.IntegrationSink) {
		s.Status.SetAddresses(addr)
	}
}

func WithIntegrationSinkSpec(spec v1alpha1.IntegrationSinkSpec) IntegrationSinkOption {
	return func(s *v1alpha1.IntegrationSink) {
		s.Spec = spec
	}
}

func WithIntegrationSinkEventPoliciesReadyBecauseOIDCDisabled() IntegrationSinkOption {
	return func(s *v1alpha1.IntegrationSink) {
		s.Status.MarkEventPoliciesTrueWithReason("OIDCDisabled", "Feature %q must be enabled to support Authorization", feature.OIDCAuthentication)
	}
}

func WithIntegrationSinkEventPoliciesReadyBecauseNoPolicyAndOIDCEnabled() IntegrationSinkOption {
	return func(s *v1alpha1.IntegrationSink) {
		s.Status.MarkEventPoliciesTrueWithReason("DefaultAuthorizationMode", "Default authz mode is %q", feature.AuthorizationAllowSameNamespace)
	}
}

func WithIntegrationSinkTrustBundlePropagatedReady() IntegrationSinkOption {
	return func(s *v1alpha1.IntegrationSink) {
		s.Status.MarkTrustBundlePropagated()
	}
}

func WithIntegrationSinkEventPoliciesReady(reason, message string) IntegrationSinkOption {
	return func(s *v1alpha1.IntegrationSink) {
		s.Status.MarkEventPoliciesTrueWithReason(reason, message)
	}
}
