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

	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
	"knative.dev/pkg/apis"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/tracker"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	containerSourceName = "test-name"
	containerSourceUID  = "uid"
)

func TestMakeSinkBinding(t *testing.T) {
	source := &v1alpha2.ContainerSource{
		ObjectMeta: metav1.ObjectMeta{Name: containerSourceName, Namespace: "test-namespace", UID: containerSourceUID},
		Spec: v1alpha2.ContainerSourceSpec{
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
	}

	want := &v1alpha2.SinkBinding{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(source),
			},
			Name:      fmt.Sprintf("%s-sinkbinding", source.Name),
			Namespace: source.Namespace,
		},
		Spec: v1alpha2.SinkBindingSpec{
			SourceSpec: source.Spec.SourceSpec,
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					APIVersion: subjectGVK.GroupVersion().String(),
					Kind:       subjectGVK.Kind,
					Namespace:  source.Namespace,
					Name:       DeploymentName(source),
				},
			},
		},
	}

	got := MakeSinkBinding(source)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}

}
