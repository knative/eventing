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
type ReceiveAdapterArgs struct {
	Image   string
	Source  *v1alpha1.CronJobSource
	Labels  map[string]string
	SinkURI string
}

// MakeReceiveAdapter generates (but does not insert into K8s) the Receive Adapter Deployment for
// Cron Job Sources.
func MakeReceiveAdapter(args *ReceiveAdapterArgs) *v1.Deployment {
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
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    RequestResourceCPU,
			corev1.ResourceMemory: RequestResourceMemory,
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    LimitResourceCPU,
			corev1.ResourceMemory: LimitResourceMemory,
		},
	}

	return &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Source.Namespace,
			Name:      utils.GenerateFixedName(args.Source, fmt.Sprintf("cronjobsource-%s", args.Source.Name)),
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
									Name:  "SINK_URI",
									Value: args.SinkURI,
								},
								{
									Name:  "NAME",
									Value: args.Source.Name,
								},
								{
									Name:  "NAMESPACE",
									Value: args.Source.Namespace,
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
