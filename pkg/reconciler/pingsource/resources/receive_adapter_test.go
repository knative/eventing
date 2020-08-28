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
	"testing"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"

	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
)

func TestMakeReceiveAdapter(t *testing.T) {
	src := &v1alpha2.PingSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-name",
			Namespace: "source-namespace",
			UID:       "source-uid",
		},
		Spec: v1alpha2.PingSourceSpec{
			Schedule: "*/2 * * * *",
			JsonData: "data",
		},
	}

	exampleUri, _ := apis.ParseURL("uri://example")

	got := MakeReceiveAdapter(&Args{
		Image:  "test-image",
		Source: src,
		Labels: map[string]string{
			"test-key1": "test-value1",
			"test-key2": "test-value2",
		},
		SinkURI: exampleUri,
	})

	one := int32(1)
	yes := true
	want := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "source-namespace",
			Name:      fmt.Sprintf("pingsource-%s-%s", src.Name, src.UID),
			Labels: map[string]string{
				"test-key1": "test-value1",
				"test-key2": "test-value2",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "sources.knative.dev/v1alpha2",
				Kind:               "PingSource",
				Name:               src.Name,
				UID:                src.UID,
				Controller:         &yes,
				BlockOwnerDeletion: &yes,
			}},
		},
		Spec: v1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-key1": "test-value1",
					"test-key2": "test-value2",
				},
			},
			Replicas: &one,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"test-key1": "test-value1",
						"test-key2": "test-value2",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "pingsource-source-name-source-uid",
					Containers: []corev1.Container{
						{
							Name:  "receive-adapter",
							Image: "test-image",
							Ports: []corev1.ContainerPort{{
								Name:          "metrics",
								ContainerPort: 9090,
							}},
							Env: []corev1.EnvVar{
								{
									Name:  "SCHEDULE",
									Value: "*/2 * * * *",
								},
								{
									Name:  "DATA",
									Value: "data",
								},
								{
									Name:  "K_SINK",
									Value: "uri://example",
								},
								{
									Name:  "NAME",
									Value: "source-name",
								},
								{
									Name:  "NAMESPACE",
									Value: "source-namespace",
								},
								{
									Name:  "METRICS_DOMAIN",
									Value: "knative.dev/eventing",
								},
								{
									Name:  "K_METRICS_CONFIG",
									Value: "",
								},
								{
									Name:  "K_LOGGING_CONFIG",
									Value: "",
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("20m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("32Mi"),
								},
							},
						},
					},
				},
			},
		},
	}

	if diff, err := kmp.SafeDiff(want, got); err != nil {
		t.Errorf("unexpected cron job resources (-want, +got) = %v", err)
	} else if diff != "" {
		t.Errorf("Unexpected deployment (-want +got) = %v", diff)
	}
}
