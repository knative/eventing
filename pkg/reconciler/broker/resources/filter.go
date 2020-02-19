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
	filterContainerName = "filter"
)

// FilterArgs are the arguments to create a Broker's filter Deployment.
type FilterArgs struct {
	Broker             *eventingv1alpha1.Broker
	Image              string
	ServiceAccountName string
}

func MakeFilterServiceMeta(b *eventingv1alpha1.Broker) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: b.Namespace,
		Name:      fmt.Sprintf("%s-broker-filter", b.Name),
		Labels:    FilterLabels(b.Name),
	}
}

// MakeFilterServiceArgs creates the in-memory representation of the Broker's filter service arguments.
func MakeFilterServiceArgs(args *FilterArgs) *service.Args {
	return &service.Args{
		ServiceMeta: MakeFilterServiceMeta(args.Broker),
		DeployMeta:  MakeFilterServiceMeta(args.Broker),
		PodSpec: corev1.PodSpec{
			ServiceAccountName: args.ServiceAccountName,
			Containers: []corev1.Container{
				{
					Name:  filterContainerName,
					Image: args.Image,
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
					ReadinessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/readyz",
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
							Value: fmt.Sprintf("%s-broker-filter", args.Broker.Name),
						},
						{
							Name:  "CONTAINER_NAME",
							Value: filterContainerName,
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

// FilterLabels generates the labels present on all resources representing the filter of the given
// Broker.
func FilterLabels(brokerName string) map[string]string {
	return map[string]string{
		eventing.BrokerLabelKey:           brokerName,
		"eventing.knative.dev/brokerRole": "filter",
	}
}
