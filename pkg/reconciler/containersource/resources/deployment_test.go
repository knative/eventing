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

	v1 "knative.dev/eventing/pkg/apis/sources/v1"

	"github.com/google/go-cmp/cmp"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	name = "test-name"
	uid  = "uid"
)

func TestMakeDeployment(t *testing.T) {
	yes := true
	tests := []struct {
		name   string
		source *v1.ContainerSource
		want   *appsv1.Deployment
	}{
		{
			name: "valid container source with one container",
			source: &v1.ContainerSource{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test-namespace", UID: uid},
				Spec: v1.ContainerSourceSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							ServiceAccountName: "test-service-account",
							Containers: []corev1.Container{
								{
									Name:  "test-source",
									Image: "test-image",
									Args:  []string{"--test1=args1", "--test2=args2"},
									Env: []corev1.EnvVar{
										{
											Name:  "test1",
											Value: "arg1",
										},
										{
											Name: "test2",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													Key: "test2-secret",
												},
											},
										},
									},
									ImagePullPolicy: corev1.PullIfNotPresent,
								},
							},
						},
					},
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							URI: apis.HTTP("test-sink"),
						},
					},
				},
			},
			want: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-deployment", name),
					Namespace: "test-namespace",
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion:         "sources.knative.dev/v1",
						Kind:               "ContainerSource",
						Name:               name,
						UID:                uid,
						Controller:         &yes,
						BlockOwnerDeletion: &yes,
					}},
					Labels: map[string]string{
						"sources.knative.dev/containerSource": name,
						"sources.knative.dev/source":          "container-source-controller",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"sources.knative.dev/containerSource": name,
							"sources.knative.dev/source":          "container-source-controller",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"sources.knative.dev/containerSource": name,
								"sources.knative.dev/source":          "container-source-controller",
							},
						},
						Spec: corev1.PodSpec{
							ServiceAccountName: "test-service-account",
							Containers: []corev1.Container{
								{
									Name:  "test-source",
									Image: "test-image",
									Args: []string{
										"--test1=args1",
										"--test2=args2",
									},
									Env: []corev1.EnvVar{
										{
											Name:  "test1",
											Value: "arg1",
										}, {
											Name: "test2",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													Key: "test2-secret",
												},
											},
										}},
									ImagePullPolicy: corev1.PullIfNotPresent,
								},
							},
						},
					},
				},
			},
		},

		{
			name: "valid container source with two containers",
			source: &v1.ContainerSource{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test-namespace", UID: uid},
				Spec: v1.ContainerSourceSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							ServiceAccountName: "test-service-account",
							Containers: []corev1.Container{
								{
									Image: "test-image",
									Args:  []string{"--test1=args1", "--test2=args2"},
									Env: []corev1.EnvVar{
										{
											Name:  "test1",
											Value: "arg1",
										},
										{
											Name: "test2",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													Key: "test2-secret",
												},
											},
										},
									},
									ImagePullPolicy: corev1.PullIfNotPresent,
								},
								{
									Image: "test-image2",
									Args:  []string{"--test3=args3", "--test4=args4"},
									Env: []corev1.EnvVar{
										{
											Name:  "test3",
											Value: "arg3",
										},
										{
											Name: "test4",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													Key: "test4-secret",
												},
											},
										},
									},
								},
							},
						},
					},
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							URI: apis.HTTP("test-sink"),
						},
					},
				},
			},
			want: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-deployment", name),
					Namespace: "test-namespace",
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion:         "sources.knative.dev/v1",
						Kind:               "ContainerSource",
						Name:               name,
						UID:                uid,
						Controller:         &yes,
						BlockOwnerDeletion: &yes,
					}},
					Labels: map[string]string{
						"sources.knative.dev/containerSource": name,
						"sources.knative.dev/source":          "container-source-controller",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"sources.knative.dev/containerSource": name,
							"sources.knative.dev/source":          "container-source-controller",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"sources.knative.dev/containerSource": name,
								"sources.knative.dev/source":          "container-source-controller",
							},
						},
						Spec: corev1.PodSpec{
							ServiceAccountName: "test-service-account",
							Containers: []corev1.Container{
								{
									Image: "test-image",
									Args: []string{
										"--test1=args1",
										"--test2=args2",
									},
									Env: []corev1.EnvVar{
										{
											Name:  "test1",
											Value: "arg1",
										}, {
											Name: "test2",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													Key: "test2-secret",
												},
											},
										},
									},
									ImagePullPolicy: corev1.PullIfNotPresent,
								},
								{
									Image: "test-image2",
									Args: []string{
										"--test3=args3",
										"--test4=args4",
									},
									Env: []corev1.EnvVar{
										{
											Name:  "test3",
											Value: "arg3",
										},
										{
											Name: "test4",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													Key: "test4-secret",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.source.Labels = Labels(name)
			got := MakeDeployment(test.source)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Error("unexpected deploy (-want, +got) =", diff)
			}
		})
	}
}
