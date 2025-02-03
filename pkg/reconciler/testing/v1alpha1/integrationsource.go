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

	duckv1 "knative.dev/pkg/apis/duck/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	v1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
)

// IntegrationSourceOption enables further configuration of a IntegrationSource.
type IntegrationSourceOption func(source *v1alpha1.IntegrationSource)

// NewIntegrationSource creates a v1 IntegrationSource with IntegrationSourceOptions
func NewIntegrationSource(name, namespace string, o ...IntegrationSourceOption) *v1alpha1.IntegrationSource {
	s := &v1alpha1.IntegrationSource{
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

func WithIntegrationSourceUID(uid types.UID) IntegrationSourceOption {
	return func(s *v1alpha1.IntegrationSource) {
		s.UID = uid
	}
}

// WithInitIntegrationSourceConditions initializes the IntegrationSource's conditions.
func WithInitIntegrationSourceConditions(s *v1alpha1.IntegrationSource) {
	s.Status.InitializeConditions()
}

func WithIntegrationSourceStatusObservedGeneration(generation int64) IntegrationSourceOption {
	return func(s *v1alpha1.IntegrationSource) {
		s.Status.ObservedGeneration = generation
	}
}

func WithIntegrationSourcePropagateContainerSourceStatus(status *v1.ContainerSourceStatus) IntegrationSourceOption {
	return func(s *v1alpha1.IntegrationSource) {
		s.Status.PropagateContainerSourceStatus(status)
	}
}

func WithIntegrationSourceSpec(spec v1alpha1.IntegrationSourceSpec) IntegrationSourceOption {
	return func(s *v1alpha1.IntegrationSource) {
		s.Spec = spec
	}
}

func WithIntegrationSourceOIDCServiceAccountName(name string) IntegrationSourceOption {
	return func(s *v1alpha1.IntegrationSource) {
		if s.Status.Auth == nil {
			s.Status.Auth = &duckv1.AuthStatus{}
		}

		s.Status.Auth.ServiceAccountName = &name
	}
}
