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
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EndpointSliceOption enables further configuration of an EndpointSlice.
type EndpointSliceOption func(*discoveryv1.EndpointSlice)

// NewEndpointSlice creates an EndpointSlice with EndpointSliceOptions
func NewEndpointSlice(name, namespace string, so ...EndpointSliceOption) *discoveryv1.EndpointSlice {
	s := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		AddressType: discoveryv1.AddressTypeIPv4,
	}
	for _, opt := range so {
		opt(s)
	}
	return s
}

func WithEndpointSliceLabels(labels map[string]string) EndpointSliceOption {
	return func(s *discoveryv1.EndpointSlice) {
		s.ObjectMeta.Labels = labels
	}
}

func WithEndpointSliceAddresses(addrs ...discoveryv1.Endpoint) EndpointSliceOption {
	return func(s *discoveryv1.EndpointSlice) {
		s.Endpoints = addrs
	}
}

func WithEndpointSliceAnnotations(annotations map[string]string) EndpointSliceOption {
	return func(s *discoveryv1.EndpointSlice) {
		s.ObjectMeta.Annotations = annotations
	}
}
