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

package resources

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// IngressArgs are the arguments to create a Broker's ingress Deployment.
type IngressArgs struct {
	Broker             *eventingv1alpha1.Broker
	Image              string
	ServiceAccountName string
	ChannelAddress     string
}

// MakeIngress creates the in-memory representation of the Broker's ingress Deployment.
func MakeIngress(args *IngressArgs) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Broker.Namespace,
			Name:      fmt.Sprintf("%s-broker-ingress", args.Broker.Name),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(args.Broker, schema.GroupVersionKind{
					Group:   eventingv1alpha1.SchemeGroupVersion.Group,
					Version: eventingv1alpha1.SchemeGroupVersion.Version,
					Kind:    "Broker",
				}),
			},
			Labels: IngressLabels(args.Broker.Name),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: IngressLabels(args.Broker.Name),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: IngressLabels(args.Broker.Name),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: args.ServiceAccountName,
					Containers: []corev1.Container{
						{
							Image: args.Image,
							Name:  "ingress",
							Env: []corev1.EnvVar{
								{
									Name:  "FILTER",
									Value: "", // TODO Add one.
								},
								{
									Name:  "CHANNEL",
									Value: args.ChannelAddress,
								},
								{
									Name:  "BROKER",
									Value: args.Broker.Name,
								},
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8080,
									Name:          "http",
								},
								{
									ContainerPort: 9090,
									Name:          "metrics",
								},
							},
						},
					},
				},
			},
		},
	}
}

// MakeIngressService creates the in-memory representation of the Broker's ingress Service.
func MakeIngressService(b *eventingv1alpha1.Broker) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: b.Namespace,
			// TODO add -ingress to the name to be consistent with the filter service naming.
			Name:   fmt.Sprintf("%s-broker", b.Name),
			Labels: IngressLabels(b.Name),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(b, schema.GroupVersionKind{
					Group:   eventingv1alpha1.SchemeGroupVersion.Group,
					Version: eventingv1alpha1.SchemeGroupVersion.Version,
					Kind:    "Broker",
				}),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: IngressLabels(b.Name),
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
				{
					Name: "metrics",
					Port: 9090,
				},
			},
		},
	}
}

// IngressLabels generates the labels present on all resources representing the ingress of the given
// Broker.
func IngressLabels(brokerName string) map[string]string {
	return map[string]string{
		"eventing.knative.dev/broker":     brokerName,
		"eventing.knative.dev/brokerRole": "ingress",
	}
}
