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

// DeploymentOption enables further configuration of a Deployment.
type DeploymentOption func(*appsv1.Deployment)

// NewDeployment creates a Deployment with DeploymentOptions.
func NewDeployment(name, namespace string, do ...DeploymentOption) *appsv1.Deployment {
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{}},
				},
			},
		},
	}
	for _, opt := range do {
		opt(d)
	}
	return d
}

func WithDeploymentLabels(labels map[string]string) DeploymentOption {
	return func(d *appsv1.Deployment) {
		d.ObjectMeta.Labels = labels
		d.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: labels,
		}
		d.Spec.Template.Labels = labels
	}
}

func WithDeploymentOwnerReferences(ownerReferences []metav1.OwnerReference) DeploymentOption {
	return func(d *appsv1.Deployment) {
		d.OwnerReferences = ownerReferences
	}
}

func WithDeploymentAnnotations(annotations map[string]string) DeploymentOption {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Annotations = annotations
	}
}

func WithDeploymentServiceAccount(serviceAccountName string) DeploymentOption {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.ServiceAccountName = serviceAccountName
	}
}

func WithDeploymentContainer(name, image string, liveness *corev1.Probe, readiness *corev1.Probe, envVars []corev1.EnvVar, containerPorts []corev1.ContainerPort) DeploymentOption {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers[0].Name = name
		d.Spec.Template.Spec.Containers[0].Image = image
		d.Spec.Template.Spec.Containers[0].LivenessProbe = liveness
		d.Spec.Template.Spec.Containers[0].ReadinessProbe = readiness
		d.Spec.Template.Spec.Containers[0].Env = envVars
		d.Spec.Template.Spec.Containers[0].Ports = containerPorts
	}
}

// WithDeploymentAvailable marks the Deployment as available.
func WithDeploymentAvailable() DeploymentOption {
	return func(d *appsv1.Deployment) {
		d.Status.Conditions = []appsv1.DeploymentCondition{
			{
				Type:   appsv1.DeploymentAvailable,
				Status: corev1.ConditionTrue,
			},
		}
	}
}
