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
	"testing"

	"github.com/knative/eventing/pkg/apis/sources/v1alpha1"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	name = "test-name"
	uid  = "uid"
)

func TestMakeDeployment_template(t *testing.T) {
	yes := true
	tests := []struct {
		name string
		args ContainerArguments
		want *appsv1.Deployment
	}{
		{
			name: "sink override and annotation label not allowed",
			args: ContainerArguments{
				Source: &v1alpha1.ContainerSource{
					ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid},
				},
				Name:      name,
				Namespace: "test-namespace",
				Template: &corev1.PodTemplateSpec{
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
				Sink: "test-sink",
				Labels: map[string]string{
					"sources.eventing.knative.dev/containerSource": "not-allowed",
					"anotherlabel":                                 "extra-label",
				},
				Annotations: map[string]string{
					"sidecar.istio.io/inject": "false",
					"anotherannotation":       "extra-annotation",
				},
			},
			want: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("containersource-%s-%s", name, uid),
					Namespace: "test-namespace",
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion:         "sources.eventing.knative.dev/v1alpha1",
						Kind:               "ContainerSource",
						Name:               name,
						UID:                uid,
						Controller:         &yes,
						BlockOwnerDeletion: &yes,
					}},
					Labels: map[string]string{
						"sources.eventing.knative.dev/containerSource": name,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"sources.eventing.knative.dev/containerSource": name,
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"sidecar.istio.io/inject": "false",
								"anotherannotation":       "extra-annotation",
							},
							Labels: map[string]string{
								"sources.eventing.knative.dev/containerSource": name,
								"anotherlabel":                                 "extra-label",
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
										"--sink=test-sink",
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
										}, {
											Name:  "SINK",
											Value: "test-sink",
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
			name: "sink",
			args: ContainerArguments{
				Source: &v1alpha1.ContainerSource{
					ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid},
				},
				Name:      name,
				Namespace: "test-namespace",
				Template: &corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						ServiceAccountName: "test-service-account",
						Containers: []corev1.Container{
							{
								Image: "test-image1",
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
				Sink: "test-sink",
			},
			want: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("containersource-%s-%s", name, uid),
					Namespace: "test-namespace",
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion:         "sources.eventing.knative.dev/v1alpha1",
						Kind:               "ContainerSource",
						Name:               name,
						UID:                uid,
						Controller:         &yes,
						BlockOwnerDeletion: &yes,
					}},
					Labels: map[string]string{
						"sources.eventing.knative.dev/containerSource": name,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"sources.eventing.knative.dev/containerSource": name,
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"sources.eventing.knative.dev/containerSource": name,
							},
						},
						Spec: corev1.PodSpec{
							ServiceAccountName: "test-service-account",
							Containers: []corev1.Container{
								{
									Image: "test-image1",
									Args: []string{
										"--test1=args1",
										"--test2=args2",
										"--sink=test-sink",
									},
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
										{
											Name:  "SINK",
											Value: "test-sink",
										},
									},
								},
								{
									Image: "test-image2",
									Args: []string{
										"--test3=args3",
										"--test4=args4",
										"--sink=test-sink",
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
										{
											Name:  "SINK",
											Value: "test-sink",
										},
									},
								},
							},
						},
					},
				},
			},
		},

		{
			name: "sink in args",
			args: ContainerArguments{
				Source: &v1alpha1.ContainerSource{
					ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid},
				},
				Name:      name,
				Namespace: "test-namespace",
				Template: &corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						ServiceAccountName: "test-service-account",
						Containers: []corev1.Container{
							{
								Image: "test-image1",
								Args:  []string{"--test1=args1", "--test2=args2", "--sink=test-sink1"},
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
							},
							{
								Image: "test-image2",
								Args:  []string{"--test3=args3", "--test4=args4", "--sink=test-sink2"},
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
				Labels: map[string]string{"sources.eventing.knative.dev/containerSource": name},
			},
			want: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("containersource-%s-%s", name, uid),
					Namespace: "test-namespace",
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion:         "sources.eventing.knative.dev/v1alpha1",
						Kind:               "ContainerSource",
						Name:               name,
						UID:                uid,
						Controller:         &yes,
						BlockOwnerDeletion: &yes,
					}},
					Labels: map[string]string{
						"sources.eventing.knative.dev/containerSource": name,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"sources.eventing.knative.dev/containerSource": name,
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"sources.eventing.knative.dev/containerSource": name,
							},
						},
						Spec: corev1.PodSpec{
							ServiceAccountName: "test-service-account",
							Containers: []corev1.Container{
								{
									Image: "test-image1",
									Args: []string{
										"--test1=args1",
										"--test2=args2",
										"--sink=test-sink1",
									},
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
										{
											Name:  "SINK",
											Value: "test-sink1",
										},
									},
								},
								{
									Image: "test-image2",
									Args: []string{
										"--test3=args3",
										"--test4=args4",
										"--sink=test-sink2",
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
										{
											Name:  "SINK",
											Value: "test-sink2",
										},
									},
								},
							},
						},
					},
				},
			},
		},

		{
			name: "template takes precedence",
			args: ContainerArguments{
				Source: &v1alpha1.ContainerSource{
					ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid},
				},
				Name:      name,
				Namespace: "test-namespace",
				Template: &corev1.PodTemplateSpec{
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
							},
						},
					},
				},
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
				ServiceAccountName: "test-service-account2",
				Sink:               "test-sink",
			},
			want: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("containersource-%s-%s", name, uid),
					Namespace: "test-namespace",
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion:         "sources.eventing.knative.dev/v1alpha1",
						Kind:               "ContainerSource",
						Name:               name,
						UID:                uid,
						Controller:         &yes,
						BlockOwnerDeletion: &yes,
					}},
					Labels: map[string]string{
						"sources.eventing.knative.dev/containerSource": name,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"sources.eventing.knative.dev/containerSource": name,
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"sources.eventing.knative.dev/containerSource": name,
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
										"--sink=test-sink",
									},
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
										{
											Name:  "SINK",
											Value: "test-sink",
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
		tt := test
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := MakeDeployment(tt.args)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("unexpected deploy (-want, +got) = %v", diff)
			}
		})
	}
}

func TestMakeDeployment_sinkoverrideannotationlabelnotallowed(t *testing.T) {
	yes := true
	got := MakeDeployment(ContainerArguments{
		Source: &v1alpha1.ContainerSource{
			ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid},
		},
		Name:      name,
		Namespace: "test-namespace",
		Image:     "test-image",
		Args:      []string{"--test1=args1", "--test2=args2"},
		Env: []corev1.EnvVar{{
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
		ServiceAccountName: "test-service-account",
		Sink:               "test-sink",
		Labels: map[string]string{
			"sources.eventing.knative.dev/containerSource": "not-allowed",
			"anotherlabel":                                 "extra-label",
		},
		Annotations: map[string]string{
			"sidecar.istio.io/inject": "false",
			"anotherannotation":       "extra-annotation",
		},
	})

	want := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("containersource-%s-%s", name, uid),
			Namespace: "test-namespace",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "sources.eventing.knative.dev/v1alpha1",
				Kind:               "ContainerSource",
				Name:               name,
				UID:                uid,
				Controller:         &yes,
				BlockOwnerDeletion: &yes,
			}},
			Labels: map[string]string{
				"sources.eventing.knative.dev/containerSource": name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"sources.eventing.knative.dev/containerSource": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "false",
						"anotherannotation":       "extra-annotation",
					},
					Labels: map[string]string{
						"sources.eventing.knative.dev/containerSource": name,
						"anotherlabel":                                 "extra-label",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "test-service-account",
					Containers: []corev1.Container{
						{
							Name:  "source",
							Image: "test-image",
							Args: []string{
								"--test1=args1",
								"--test2=args2",
								"--sink=test-sink",
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
								}, {
									Name:  "SINK",
									Value: "test-sink",
								}},
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
					},
				},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected deploy (-want, +got) = %v", diff)
	}
}

func TestMakeDeployment_sink(t *testing.T) {
	yes := true
	got := MakeDeployment(ContainerArguments{
		Source: &v1alpha1.ContainerSource{
			ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid},
		},
		Name:      name,
		Namespace: "test-namespace",
		Image:     "test-image",
		Args:      []string{"--test1=args1", "--test2=args2"},
		Env: []corev1.EnvVar{{
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
		ServiceAccountName: "test-service-account",
		Sink:               "test-sink",
	})
	want := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("containersource-%s-%s", name, uid),
			Namespace: "test-namespace",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "sources.eventing.knative.dev/v1alpha1",
				Kind:               "ContainerSource",
				Name:               name,
				UID:                uid,
				Controller:         &yes,
				BlockOwnerDeletion: &yes,
			}},
			Labels: map[string]string{
				"sources.eventing.knative.dev/containerSource": name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"sources.eventing.knative.dev/containerSource": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"sources.eventing.knative.dev/containerSource": name,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "test-service-account",
					Containers: []corev1.Container{
						{
							Name:  "source",
							Image: "test-image",
							Args: []string{
								"--test1=args1",
								"--test2=args2",
								"--sink=test-sink",
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
								}, {
									Name:  "SINK",
									Value: "test-sink",
								}},
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
					},
				},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected deploy (-want, +got) = %v", diff)
	}
}

func TestMakeDeployment_sinkinargs(t *testing.T) {
	yes := true
	got := MakeDeployment(ContainerArguments{
		Source: &v1alpha1.ContainerSource{
			ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid},
		},
		Name:      name,
		Namespace: "test-namespace",
		Image:     "test-image",
		Args:      []string{"--test1=args1", "--test2=args2", "--sink=test-sink"},
		Env: []corev1.EnvVar{{
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
		ServiceAccountName: "test-service-account",
		Labels:             map[string]string{"sources.eventing.knative.dev/containerSource": name},
	})

	want := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("containersource-%s-%s", name, uid),
			Namespace: "test-namespace",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "sources.eventing.knative.dev/v1alpha1",
				Kind:               "ContainerSource",
				Name:               name,
				UID:                uid,
				Controller:         &yes,
				BlockOwnerDeletion: &yes,
			}},
			Labels: map[string]string{
				"sources.eventing.knative.dev/containerSource": name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"sources.eventing.knative.dev/containerSource": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"sources.eventing.knative.dev/containerSource": name,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "test-service-account",
					Containers: []corev1.Container{
						{
							Name:  "source",
							Image: "test-image",
							Args: []string{
								"--test1=args1",
								"--test2=args2",
								"--sink=test-sink",
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
								}, {
									Name:  "SINK",
									Value: "test-sink",
								}},
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
					},
				},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected deploy (-want, +got) = %v", diff)
	}
}
