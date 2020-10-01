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

package v1alpha1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/tracker"
)

func TestSinkBindingDefaulting(t *testing.T) {
	tests := []struct {
		name string
		in   *SinkBinding
		want *SinkBinding
	}{{
		name: "namespace is defaulted",
		in: &SinkBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "matt",
				Namespace: "moore",
			},
			Spec: SinkBindingSpec{
				BindingSpec: duckv1alpha1.BindingSpec{
					Subject: tracker.Reference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "jeanne",
					},
				},
				SourceSpec: duckv1.SourceSpec{
					Sink: duckv1.Destination{
						Ref: &duckv1.KReference{
							APIVersion: "serving.knative.dev/v1",
							Kind:       "Service",
							Name:       "gemma",
						},
					},
				},
			},
		},
		want: &SinkBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "matt",
				Namespace: "moore",
			},
			Spec: SinkBindingSpec{
				BindingSpec: duckv1alpha1.BindingSpec{
					Subject: tracker.Reference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "jeanne",
						// This is filled in by defaulting.
						Namespace: "moore",
					},
				},
				SourceSpec: duckv1.SourceSpec{
					Sink: duckv1.Destination{
						Ref: &duckv1.KReference{
							APIVersion: "serving.knative.dev/v1",
							Kind:       "Service",
							Name:       "gemma",
							// This is filled in by defaulting.
							Namespace: "moore",
						},
					},
				},
			},
		},
	}, {
		name: "no ref, given namespace",
		in: &SinkBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "matt",
				Namespace: "moore",
			},
			Spec: SinkBindingSpec{
				BindingSpec: duckv1alpha1.BindingSpec{
					Subject: tracker.Reference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "jeanne",
						Namespace:  "lorefice",
					},
				},
				SourceSpec: duckv1.SourceSpec{
					Sink: duckv1.Destination{
						URI: &apis.URL{
							Scheme: "http",
							Host:   "moore.dev",
						},
					},
				},
			},
		},
		want: &SinkBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "matt",
				Namespace: "moore",
			},
			Spec: SinkBindingSpec{
				BindingSpec: duckv1alpha1.BindingSpec{
					Subject: tracker.Reference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "jeanne",
						Namespace:  "lorefice",
					},
				},
				SourceSpec: duckv1.SourceSpec{
					Sink: duckv1.Destination{
						URI: &apis.URL{
							Scheme: "http",
							Host:   "moore.dev",
						},
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.in
			got.SetDefaults(context.Background())
			if !cmp.Equal(test.want, got) {
				t.Error("SetDefaults (-want, +got) =", cmp.Diff(test.want, got))
			}
		})
	}
}
