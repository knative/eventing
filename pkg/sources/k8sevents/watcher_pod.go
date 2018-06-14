/*
Copyright 2018 Google LLC

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

package main

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MakeWatcherDeployment creates a deployment for a watcher.
// TODO: a whole bunch...
func MakeWatcherDeployment(namespace string, deploymentName string, serviceAccount string, image string, eventNamespace string, route string) *appsv1.Deployment {
	replicas := int32(1)
	labels := map[string]string{
		"watcher": deploymentName,
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccount,
					Containers: []corev1.Container{
						corev1.Container{
							Name:            deploymentName,
							Image:           image,
							ImagePullPolicy: "Always",
							Env: []corev1.EnvVar{
								{
									Name:  "TARGET",
									Value: route,
								},
								{
									Name:  "NAMESPACE",
									Value: eventNamespace,
								},
							},
						},
					},
				},
			},
		},
	}
}
