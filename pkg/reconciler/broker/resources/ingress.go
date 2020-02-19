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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/reconciler/internal/service"
	"knative.dev/pkg/system"
)

const (
	ingressContainerName = "ingress"
)

// IngressArgs are the arguments to create a Broker's ingress Deployment.
type IngressArgs struct {
	Broker             *eventingv1alpha1.Broker
	Image              string
	ServiceAccountName string
	ChannelAddress     string
}

// MakeIngressServiceArgs creates the in-memory representation of the Broker's ingress service arguments.
func MakeIngressServiceArgs(args *IngressArgs) *service.Args {
	return &service.Args{
		ServiceMeta: metav1.ObjectMeta{
			Namespace: args.Broker.Namespace,
			Name:      fmt.Sprintf("%s-broker", args.Broker.Name),
			Labels:    IngressLabels(args.Broker.Name),
		},
		DeployMeta: metav1.ObjectMeta{
			Namespace: args.Broker.Namespace,
			Name:      fmt.Sprintf("%s-broker-ingress", args.Broker.Name),
			Labels:    IngressLabels(args.Broker.Name),
		},
		PodSpec: corev1.PodSpec{
			ServiceAccountName: args.ServiceAccountName,
			Containers: []corev1.Container{
				{
					Image: args.Image,
					Name:  ingressContainerName,
					LivenessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/healthz",
								// Port should be the same as the container port.
							},
						},
						InitialDelaySeconds: 5,
						PeriodSeconds:       2,
						FailureThreshold:    3,
						TimeoutSeconds:      10,
						SuccessThreshold:    1,
					},
					Env: []corev1.EnvVar{
						{
							Name:  system.NamespaceEnvKey,
							Value: system.Namespace(),
						},
						{
							Name:  "NAMESPACE",
							Value: args.Broker.Namespace,
						},
						{
							Name:  "POD_NAME",
							Value: fmt.Sprintf("%s-broker-ingress", args.Broker.Name),
						},
						{
							Name:  "CONTAINER_NAME",
							Value: ingressContainerName,
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
						// Used for StackDriver only.
						{
							Name:  "METRICS_DOMAIN",
							Value: "knative.dev/internal/eventing",
						},
					},
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 8080,
						},
					},
				},
			},
		},
	}
}

// IngressLabels generates the labels present on all resources representing the ingress of the given
// Broker.
func IngressLabels(brokerName string) map[string]string {
	return map[string]string{
		eventing.BrokerLabelKey:           brokerName,
		"eventing.knative.dev/brokerRole": "ingress",
	}
}
