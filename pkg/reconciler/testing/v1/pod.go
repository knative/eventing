/*
Copyright 2025 The Knative Authors

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

// PodOption enables further configuration of a Pod.
type PodOption func(*corev1.Pod)

// NewPod creates a Pod with PodOptions.
func NewPod(name, namespace string, po ...PodOption) *corev1.Pod {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "container",
					Image: "test-image",
				},
			},
		},
	}
	for _, opt := range po {
		opt(p)
	}
	return p
}

func WithPodLabels(labels map[string]string) PodOption {
	return func(p *corev1.Pod) {
		p.ObjectMeta.Labels = labels
	}
}

func WithPodOwnerReferences(ownerReferences []metav1.OwnerReference) PodOption {
	return func(p *corev1.Pod) {
		p.OwnerReferences = ownerReferences
	}
}

func WithPodAnnotations(annotations map[string]string) PodOption {
	return func(p *corev1.Pod) {
		p.ObjectMeta.Annotations = annotations
	}
}

func WithPodIP(ip string) PodOption {
	return func(p *corev1.Pod) {
		p.Status.PodIP = ip
	}
}

func WithPodReady() PodOption {
	return func(p *corev1.Pod) {
		p.Status.Phase = corev1.PodRunning
		p.Status.Conditions = []corev1.PodCondition{
			{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			},
		}
	}
}

func WithPodContainer(name, image string) PodOption {
	return func(p *corev1.Pod) {
		if len(p.Spec.Containers) == 0 {
			p.Spec.Containers = []corev1.Container{{}}
		}
		p.Spec.Containers[0].Name = name
		p.Spec.Containers[0].Image = image
	}
}
