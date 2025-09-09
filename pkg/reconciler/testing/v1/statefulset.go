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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StatefulSetOption enables further configuration of a StatefulSet.
type StatefulSetOption func(*appsv1.StatefulSet)

// NewStatefulSet creates a StatefulSet with StatefulSetOptions.
func NewStatefulSet(name, namespace string, so ...StatefulSetOption) *appsv1.StatefulSet {
	s := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{}},
				},
			},
		},
	}
	for _, opt := range so {
		opt(s)
	}
	return s
}

func WithStatefulSetLabels(labels map[string]string) StatefulSetOption {
	return func(s *appsv1.StatefulSet) {
		s.ObjectMeta.Labels = labels
		s.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: labels,
		}
		s.Spec.Template.Labels = labels
	}
}

func WithStatefulSetOwnerReferences(ownerReferences []metav1.OwnerReference) StatefulSetOption {
	return func(s *appsv1.StatefulSet) {
		s.OwnerReferences = ownerReferences
	}
}

func WithStatefulSetAnnotations(annotations map[string]string) StatefulSetOption {
	return func(s *appsv1.StatefulSet) {
		s.Spec.Template.Annotations = annotations
	}
}

func WithStatefulSetServiceAccount(serviceAccountName string) StatefulSetOption {
	return func(s *appsv1.StatefulSet) {
		s.Spec.Template.Spec.ServiceAccountName = serviceAccountName
	}
}

func WithStatefulSetContainer(name, image string, liveness *corev1.Probe, readiness *corev1.Probe, envVars []corev1.EnvVar, containerPorts []corev1.ContainerPort) StatefulSetOption {
	return func(s *appsv1.StatefulSet) {
		s.Spec.Template.Spec.Containers[0].Name = name
		s.Spec.Template.Spec.Containers[0].Image = image
		s.Spec.Template.Spec.Containers[0].LivenessProbe = liveness
		s.Spec.Template.Spec.Containers[0].ReadinessProbe = readiness
		s.Spec.Template.Spec.Containers[0].Env = envVars
		s.Spec.Template.Spec.Containers[0].Ports = containerPorts
	}
}

// WithStatefulSetReady marks the StatefulSet as ready.
func WithStatefulSetReady() StatefulSetOption {
	return func(s *appsv1.StatefulSet) {
		s.Status.ReadyReplicas = 1
		s.Status.Replicas = 1
	}
}

func WithStatefulSetReplicas(replicas int32) StatefulSetOption {
	return func(s *appsv1.StatefulSet) {
		s.Spec.Replicas = &replicas
		s.Status.Replicas = replicas
		s.Status.ReadyReplicas = replicas
	}
}
