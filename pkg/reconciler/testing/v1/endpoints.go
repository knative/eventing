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

package testing

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EndpointsOption enables further configuration of a Endpoints.
type EndpointsOption func(*corev1.Endpoints)

// NewEndpoints creates a Endpoints with EndpointsOptions
func NewEndpoints(name, namespace string, so ...EndpointsOption) *corev1.Endpoints {
	s := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range so {
		opt(s)
	}
	return s
}

func WithEndpointsLabels(labels map[string]string) EndpointsOption {
	return func(s *corev1.Endpoints) {
		s.ObjectMeta.Labels = labels
	}
}

func WithEndpointsAddresses(addrs ...corev1.EndpointAddress) EndpointsOption {
	return func(s *corev1.Endpoints) {
		s.Subsets = []corev1.EndpointSubset{{
			Addresses: addrs,
		}}
	}
}

func WithEndpointsNotReadyAddresses(addrs ...corev1.EndpointAddress) EndpointsOption {
	return func(s *corev1.Endpoints) {
		s.Subsets = []corev1.EndpointSubset{{
			NotReadyAddresses: addrs,
		}}
	}
}

func WithEndpointsAnnotations(annotations map[string]string) EndpointsOption {
	return func(s *corev1.Endpoints) {
		s.ObjectMeta.Annotations = annotations
	}
}
