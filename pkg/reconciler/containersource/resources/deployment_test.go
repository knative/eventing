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
	"testing"

	"github.com/knative/eventing/pkg/apis/sources/v1alpha1"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMakeDeployment_sinkoverrideannotationlabelnotallowed(t *testing.T) {
	yes := true
	got := MakeDeployment(ContainerArguments{
		Source: &v1alpha1.ContainerSource{
			ObjectMeta: metav1.ObjectMeta{Name: "test-name", UID: "TEST_UID"},
		},
		Name:      "test-name",
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
		SinkInArgs:         false,
		Sink:               "test-sink",
		Labels: map[string]string{
			"eventing.knative.dev/source": "not-allowed",
			"anotherlabel":                "extra-label",
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
			GenerateName: "test-name-",
			Namespace:    "test-namespace",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "sources.eventing.knative.dev/v1alpha1",
				Kind:               "ContainerSource",
				Name:               "test-name",
				UID:                "TEST_UID",
				Controller:         &yes,
				BlockOwnerDeletion: &yes,
			}},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"eventing.knative.dev/source": "test-name",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "false",
						"anotherannotation":       "extra-annotation",
					},
					Labels: map[string]string{
						"eventing.knative.dev/source": "test-name",
						"anotherlabel":                "extra-label",
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
			ObjectMeta: metav1.ObjectMeta{Name: "test-name", UID: "TEST_UID"},
		},
		Name:      "test-name",
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
		SinkInArgs:         false,
		Sink:               "test-sink",
	})

	want := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-name-",
			Namespace:    "test-namespace",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "sources.eventing.knative.dev/v1alpha1",
				Kind:               "ContainerSource",
				Name:               "test-name",
				UID:                "TEST_UID",
				Controller:         &yes,
				BlockOwnerDeletion: &yes,
			}},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"eventing.knative.dev/source": "test-name",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "true",
					},
					Labels: map[string]string{
						"eventing.knative.dev/source": "test-name",
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
			ObjectMeta: metav1.ObjectMeta{Name: "test-name", UID: "TEST_UID"},
		},
		Name:      "test-name",
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
		SinkInArgs:         true,
		Labels:             map[string]string{"eventing.knative.dev/source": "test-name"},
		Annotations:        map[string]string{"sidecar.istio.io/inject": "true"},
	})

	want := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-name-",
			Namespace:    "test-namespace",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "sources.eventing.knative.dev/v1alpha1",
				Kind:               "ContainerSource",
				Name:               "test-name",
				UID:                "TEST_UID",
				Controller:         &yes,
				BlockOwnerDeletion: &yes,
			}},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"eventing.knative.dev/source": "test-name",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "true",
					},
					Labels: map[string]string{
						"eventing.knative.dev/source": "test-name",
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
