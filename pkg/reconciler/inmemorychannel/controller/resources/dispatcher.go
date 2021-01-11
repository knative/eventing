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
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/ptr"
	"knative.dev/pkg/system"
)

var (
	dispatcherLabels = map[string]string{
		"messaging.knative.dev/channel": "in-memory-channel",
		"messaging.knative.dev/role":    "dispatcher",
	}
)

type DispatcherArgs struct {
	ServiceAccountName  string
	DispatcherName      string
	DispatcherNamespace string
	Image               string
}

// MakeDispatcher generates the dispatcher deployment for the in-memory channel
func MakeDispatcher(args DispatcherArgs) *v1.Deployment {
	replicas := int32(1)

	return &v1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployments",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.DispatcherNamespace,
			Name:      args.DispatcherName,
		},
		Spec: v1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: dispatcherLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: dispatcherLabels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: args.ServiceAccountName,
					EnableServiceLinks: ptr.Bool(false),
					Containers: []corev1.Container{
						{
							Name:  "dispatcher",
							Image: args.Image,
							Env:   makeEnv(),

							// Set low resource requests and limits.
							// This should be configurable.
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("125m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("2200m"),
									corev1.ResourceMemory: resource.MustParse("2048Mi"),
								},
							},
							Ports: []corev1.ContainerPort{{
								Name:          "metrics",
								ContainerPort: 9090,
							}},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:    pointer.Int64Ptr(1000),
								RunAsGroup:   pointer.Int64Ptr(1000),
								RunAsNonRoot: pointer.BoolPtr(true),
							},
						},
					},
				},
			},
		},
	}
}

func makeEnv() []corev1.EnvVar {
	return []corev1.EnvVar{{
		Name:  system.NamespaceEnvKey,
		Value: system.Namespace(),
	}, {
		Name:  "METRICS_DOMAIN",
		Value: "knative.dev/inmemorychannel-dispatcher",
	}, {
		Name:  "CONFIG_OBSERVABILITY_NAME",
		Value: metrics.ConfigMapName(),
	}, {
		Name:  "CONFIG_LOGGING_NAME",
		Value: logging.ConfigMapName(),
	}, {
		Name: "NAMESPACE",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.namespace",
			},
		},
	}}
}
