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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/apis"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
)

// DeploymentOption enables further configuration of a Deployment.
type ServingServiceOption func(*servingv1.Service)

// NewDeployment creates a Deployment with DeploymentOptions.
func NewServingService(name, namespace, revisionName string, do ...ServingServiceOption) *servingv1.Service {
	svc := &servingv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: servingv1.ServiceSpec{
			ConfigurationSpec: servingv1.ConfigurationSpec{
				Template: servingv1.RevisionTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name:      revisionName,
						Namespace: namespace,
					},
					Spec: servingv1.RevisionSpec{
						PodSpec: corev1.PodSpec{
							Containers: []corev1.Container{{}},
						},
					},
				},
			},
		},
	}
	for _, opt := range do {
		opt(svc)
	}
	return svc
}

func WithServingServiceLabels(labels map[string]string) ServingServiceOption {
	return func(svc *servingv1.Service) {
		if svc.ObjectMeta.Labels == nil {
			svc.ObjectMeta.Labels = make(map[string]string)
		}
		for k, v := range labels {
			svc.ObjectMeta.Labels[k] = v
		}
	}
}

func WithServingServiceTemplateLabels(labels map[string]string) ServingServiceOption {
	return func(svc *servingv1.Service) {
		if svc.Spec.Template.Labels == nil {
			svc.Spec.Template.Labels = make(map[string]string)
		}
		for k, v := range labels {
			svc.Spec.Template.Labels[k] = v
		}
	}
}

func WithServingServiceOwnerReferences(ownerReferences []metav1.OwnerReference) ServingServiceOption {
	return func(svc *servingv1.Service) {
		svc.OwnerReferences = ownerReferences
	}
}

func WithServingServiceAnnotations(annotations map[string]string) ServingServiceOption {
	return func(svc *servingv1.Service) {
		svc.Annotations = annotations
	}
}

func WithServingServiceServiceAccount(serviceAccountName string) ServingServiceOption {
	return func(svc *servingv1.Service) {
		svc.Spec.Template.Spec.ServiceAccountName = serviceAccountName
	}
}

func WithServingServiceContainer(name, image string, liveness *corev1.Probe, readiness *corev1.Probe, envVars []corev1.EnvVar, containerPorts []corev1.ContainerPort) ServingServiceOption {
	return func(svc *servingv1.Service) {
		svc.Spec.Template.Spec.Containers[0].Name = name
		svc.Spec.Template.Spec.Containers[0].Image = image
		svc.Spec.Template.Spec.Containers[0].LivenessProbe = liveness
		svc.Spec.Template.Spec.Containers[0].ReadinessProbe = readiness
		svc.Spec.Template.Spec.Containers[0].Env = envVars
		svc.Spec.Template.Spec.Containers[0].Ports = containerPorts
	}
}

func WithServingServicePodSpec(podSpec corev1.PodSpec) ServingServiceOption {
	return func(svc *servingv1.Service) {
		svc.Spec.Template.Spec.Containers[0] = podSpec.Containers[0]
	}
}

func WithServingServiceReady() ServingServiceOption {
	return func(svc *servingv1.Service) {
		svc.Status.Conditions = []apis.Condition{
			apis.Condition{
				Type:   apis.ConditionReady,
				Status: corev1.ConditionTrue,
			},
		}
		svc.Status.URL = &apis.URL{
			Scheme: "http",
			Host:   fmt.Sprintf("%s.%s.svc.%s", svc.ObjectMeta.Name, svc.ObjectMeta.Namespace, utils.GetClusterDomainName()),
		}
	}
}
