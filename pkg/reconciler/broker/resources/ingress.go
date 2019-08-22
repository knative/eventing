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

	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/system"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
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
				*kmeta.NewControllerRef(args.Broker),
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
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       2,
							},
							Env: []corev1.EnvVar{
								{
									Name:  system.NamespaceEnvKey,
									Value: system.Namespace(),
								},
								{
									Name: "NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
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
								{
									Name:  "METRICS_DOMAIN",
									Value: "knative.dev/eventing",
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
			Name:        fmt.Sprintf("%s-broker", b.Name),
			Annotations: IngressAnnotations(),
			Labels:      IngressLabels(b.Name),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(b),
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

// IngressAnnotations generates the annotation that allow Prometheus to scrape the metrics exposed
// by this service.
func IngressAnnotations() map[string]string {
	return map[string]string{
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   "9090",
	}
}
