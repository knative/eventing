/*
Copyright 2024 The Knative Authors

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

package v1alpha1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestSetDefaults(t *testing.T) {
	testCases := map[string]struct {
		initial  JobSink
		expected JobSink
	}{
		"execution mode": {
			initial: JobSink{
				Spec: JobSinkSpec{
					Job: &batchv1.Job{
						Spec:       batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec:       corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:                     "cnt",
											Image:                    "img",
										},
										{
											Name:                     "cnt2",
											Image:                    "img2",
										},
										{
											Name:                     "cnt3",
											Image:                    "img3",
											Env: []corev1.EnvVar{
												{Name: "KNATIVE_EXECUTION_MODE", Value: "something"},
											},
										},
									},
									InitContainers: []corev1.Container{
										{
											Name:                     "cnt",
											Image:                    "img",
										},
										{
											Name:                     "cnt-ini2",
											Image:                    "img-ini2",
											Env: []corev1.EnvVar{
												{Name: "KNATIVE_EXECUTION_MODE", Value: "something"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: JobSink{
				Spec: JobSinkSpec{
					Job: &batchv1.Job{
						Spec:       batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec:       corev1.PodSpec{
									InitContainers: []corev1.Container{
										{
											Name:                     "cnt",
											Image:                    "img",
											Env: []corev1.EnvVar{
												{Name: "KNATIVE_EXECUTION_MODE", Value: "batch"},
											},
										},
										{
											Name:                     "cnt-ini2",
											Image:                    "img-ini2",
											Env: []corev1.EnvVar{
												{Name: "KNATIVE_EXECUTION_MODE", Value: "something"},
											},
										},
									},
									Containers: []corev1.Container{
										{
											Name:                     "cnt",
											Image:                    "img",
											Env: []corev1.EnvVar{
												{Name: "KNATIVE_EXECUTION_MODE", Value: "batch"},
											},
										},
										{
											Name:                     "cnt2",
											Image:                    "img2",
											Env: []corev1.EnvVar{
												{Name: "KNATIVE_EXECUTION_MODE", Value: "batch"},
											},
										},
										{
											Name:                     "cnt3",
											Image:                    "img3",
											Env: []corev1.EnvVar{
												{Name: "KNATIVE_EXECUTION_MODE", Value: "something"},
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
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			tc.initial.SetDefaults(context.TODO())
			if diff := cmp.Diff(tc.expected, tc.initial); diff != "" {
				t.Fatal("Unexpected defaults (-want, +got):", diff)
			}
		})
	}
}
