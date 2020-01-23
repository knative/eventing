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

package resources

import (
	"fmt"
	"knative.dev/pkg/apis"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/kmeta"
)

var (
	// one is a form of int32(1) that you can take the address of.
	one = int32(1)
)

// ReceiveAdapterArgs are the arguments needed to create a Cron Job Source Receive Adapter. Every
// field is required.
type Args struct {
	Image         string
	Source        *v1alpha1.PingSource
	Labels        map[string]string
	SinkURI       *apis.URL
	MetricsConfig string
	LoggingConfig string
}

// MakeReceiveAdapter generates (but does not insert into K8s) the Receive Adapter Deployment for
// PingSources.
func MakeReceiveAdapter(args *Args) *v1.Deployment {
	name := args.Source.ObjectMeta.Name
	RequestResourceCPU, err := resource.ParseQuantity(args.Source.Spec.Resources.Requests.ResourceCPU)
	if err != nil {
		RequestResourceCPU = resource.MustParse("250m")
	}
	RequestResourceMemory, err := resource.ParseQuantity(args.Source.Spec.Resources.Requests.ResourceMemory)
	if err != nil {
		RequestResourceMemory = resource.MustParse("512Mi")
	}
	LimitResourceCPU, err := resource.ParseQuantity(args.Source.Spec.Resources.Limits.ResourceCPU)
	if err != nil {
		LimitResourceCPU = resource.MustParse("250m")
	}
	LimitResourceMemory, err := resource.ParseQuantity(args.Source.Spec.Resources.Limits.ResourceMemory)
	if err != nil {
		LimitResourceMemory = resource.MustParse("512Mi")
	}

	res := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    RequestResourceCPU,
			corev1.ResourceMemory: RequestResourceMemory,
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    LimitResourceCPU,
			corev1.ResourceMemory: LimitResourceMemory,
		},
	}

	return &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Source.Namespace,
			Name:      utils.GenerateFixedName(args.Source, fmt.Sprintf("pingsource-%s", name)),
			Labels:    args.Labels,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Source),
			},
		},
		Spec: v1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: args.Labels,
			},
			Replicas: &one,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: args.Labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: args.Source.Spec.ServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:  "receive-adapter",
							Image: args.Image,
							Ports: []corev1.ContainerPort{
								{
									Name:          "metrics",
									ContainerPort: 9090,
								}},
							Env: []corev1.EnvVar{
								{
									Name:  "SCHEDULE",
									Value: args.Source.Spec.Schedule,
								},
								{
									Name:  "DATA",
									Value: args.Source.Spec.Data,
								},
								{
									Name:  "K_SINK",
									Value: args.SinkURI.String(),
								},
								{
									Name:  "NAME",
									Value: args.Source.Name,
								},
								{
									Name:  "NAMESPACE",
									Value: args.Source.Namespace,
								}, {
									Name:  "METRICS_DOMAIN",
									Value: "knative.dev/eventing",
								}, {
									Name:  "K_METRICS_CONFIG",
									Value: args.MetricsConfig,
								}, {
									Name:  "K_LOGGING_CONFIG",
									Value: args.LoggingConfig,
								},
							},
							Resources: res,
						},
					},
				},
			},
		},
	}
}
