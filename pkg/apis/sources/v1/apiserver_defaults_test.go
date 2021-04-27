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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestApiServerSourceDefaults(t *testing.T) {
	testCases := map[string]struct {
		initial  ApiServerSource
		expected ApiServerSource
	}{
		"no EventMode": {
			initial: ApiServerSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "test-namespace",
				},
				Spec: ApiServerSourceSpec{
					Resources: []APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Foo",
					}},
					ServiceAccountName: "default",
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "broker",
								Name:       "default",
							},
						},
					},
				},
			},
			expected: ApiServerSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "test-namespace",
				},
				Spec: ApiServerSourceSpec{
					EventMode: ReferenceMode,
					Resources: []APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Foo",
					}},
					ServiceAccountName: "default",
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "broker",
								Name:       "default",
							},
						},
					},
				},
			},
		},
		"no ServiceAccountName": {
			initial: ApiServerSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "test-namespace",
				},
				Spec: ApiServerSourceSpec{
					Resources: []APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Foo",
					}},
					EventMode: ReferenceMode,
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "broker",
								Name:       "default",
							},
						},
					},
				},
			},
			expected: ApiServerSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "test-namespace",
				},
				Spec: ApiServerSourceSpec{
					EventMode: ReferenceMode,
					Resources: []APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Foo",
					}},
					ServiceAccountName: "default",
					SourceSpec: duckv1.SourceSpec{
						Sink: duckv1.Destination{
							Ref: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "broker",
								Name:       "default",
							},
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
				t.Fatalf("Unexpected defaults (-want, +got): %s", diff)
			}
		})
	}
}
