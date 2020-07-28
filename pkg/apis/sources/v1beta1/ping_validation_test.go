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

package v1beta1

import (
	"context"
	"testing"

	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/go-cmp/cmp"
	"knative.dev/pkg/apis"
)

func TestPingSourceValidation(t *testing.T) {
	tests := []struct {
		name   string
		source PingSource
		want   *apis.FieldError
	}{{
		name: "valid spec",
		source: PingSource{
			Spec: PingSourceSpec{
				Schedule: "*/2 * * * *",
				SourceSpec: duckv1.SourceSpec{
					Sink: duckv1.Destination{
						Ref: &duckv1.KReference{
							APIVersion: "v1alpha1",
							Kind:       "broker",
							Name:       "default",
						},
					},
				},
			},
		},
		want: nil,
	}, {
		name: "valid spec with timezone",
		source: PingSource{
			Spec: PingSourceSpec{
				Schedule: "*/2 * * * *",
				Timezone: "Europe/Paris",
				SourceSpec: duckv1.SourceSpec{
					Sink: duckv1.Destination{
						Ref: &duckv1.KReference{
							APIVersion: "v1alpha1",
							Kind:       "broker",
							Name:       "default",
						},
					},
				},
			},
		},
		want: nil,
	}, {
		name: "valid spec with invalid timezone",
		source: PingSource{
			Spec: PingSourceSpec{
				Schedule: "*/2 * * * *",
				Timezone: "Knative/Land",
				SourceSpec: duckv1.SourceSpec{
					Sink: duckv1.Destination{
						Ref: &duckv1.KReference{
							APIVersion: "v1alpha1",
							Kind:       "broker",
							Name:       "default",
						},
					},
				},
			},
		},
		want: func() *apis.FieldError {
			var errs *apis.FieldError
			fe := apis.ErrInvalidValue("provided bad location Knative/Land: unknown time zone Knative/Land", "spec.timezone")
			errs = errs.Also(fe)
			return errs
		}(),
	}, {
		name: "empty sink",
		source: PingSource{
			Spec: PingSourceSpec{
				Schedule: "*/2 * * * *",
			},
		},
		want: func() *apis.FieldError {
			return apis.ErrGeneric("expected at least one, got none", "ref", "uri").ViaField("spec.sink")
		}(),
	}, {
		name: "invalid schedule",
		source: PingSource{
			Spec: PingSourceSpec{
				Schedule: "2",
				SourceSpec: duckv1.SourceSpec{
					Sink: duckv1.Destination{
						Ref: &duckv1.KReference{
							APIVersion: "v1alpha1",
							Kind:       "broker",
							Name:       "default",
						},
					},
				},
			},
		},
		want: func() *apis.FieldError {
			var errs *apis.FieldError
			fe := apis.ErrInvalidValue("expected exactly 5 fields, found 1: [2]", "spec.schedule")
			errs = errs.Also(fe)
			return errs
		}(),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.source.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("PingSourceSpec.Validate (-want, +got) = %v", diff)
			}
		})
	}
}
