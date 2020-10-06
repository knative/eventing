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

package v1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestContainerSourceDefaults(t *testing.T) {
	testCases := map[string]struct {
		initial  ContainerSource
		expected ContainerSource
	}{
		"no container name": {
			initial: ContainerSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "test-namespace",
				},
				Spec: ContainerSourceSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Image: "test-image",
							}},
						},
					},
				},
			},
			expected: ContainerSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "test-namespace",
				},
				Spec: ContainerSourceSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Name:  "test-name-0",
								Image: "test-image",
							}},
						},
					},
				},
			},
		},
		"one with container name one without": {
			initial: ContainerSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "test-namespace",
				},
				Spec: ContainerSourceSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Name:  "test-container",
								Image: "test-image",
							}, {
								Image: "test-another-image",
							}},
						},
					},
				},
			},
			expected: ContainerSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "test-namespace",
				},
				Spec: ContainerSourceSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Name:  "test-container",
								Image: "test-image",
							}, {
								Name:  "test-name-1",
								Image: "test-another-image",
							}},
						},
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			tc.initial.SetDefaults(context.Background())
			if diff := cmp.Diff(tc.expected, tc.initial); diff != "" {
				t.Fatal("Unexpected defaults (-want, +got):", diff)
			}
		})
	}
}
