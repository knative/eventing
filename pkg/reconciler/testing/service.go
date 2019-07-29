/*
Copyright 2019 The Knative Authors

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

// ServiceOption enables further configuration of a Service.
type ServiceOption func(*corev1.Service)

// NewService creates a Service with ServiceOptions
func NewService(name, namespace string, so ...ServiceOption) *corev1.Service {
	s := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{},
	}
	for _, opt := range so {
		opt(s)
	}
	return s
}

func WithServiceOwnerReferences(ownerReferences []metav1.OwnerReference) ServiceOption {
	return func(s *corev1.Service) {
		s.OwnerReferences = ownerReferences
	}
}

func WithServiceLabels(labels map[string]string) ServiceOption {
	return func(s *corev1.Service) {
		s.ObjectMeta.Labels = labels
		s.Spec.Selector = labels
	}
}

func WithServicePorts(ports []corev1.ServicePort) ServiceOption {
	return func(s *corev1.Service) {
		s.Spec.Ports = ports
	}
}

func WithServiceAnnotations(annotations map[string]string) ServiceOption {
	return func(s *corev1.Service) {
		s.ObjectMeta.Annotations = annotations
	}
}
