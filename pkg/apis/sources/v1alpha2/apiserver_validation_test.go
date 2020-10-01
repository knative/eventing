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

package v1alpha2

import (
	"context"
	"errors"
	"testing"

	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/go-cmp/cmp"
	"knative.dev/pkg/apis"
)

func TestAPIServerValidation(t *testing.T) {
	tests := []struct {
		name string
		spec ApiServerSourceSpec
		want error
	}{{
		name: "valid spec",
		spec: ApiServerSourceSpec{
			EventMode: "Resource",
			Resources: []APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Foo",
			}},
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
		want: nil,
	}, {
		name: "empty sink",
		spec: ApiServerSourceSpec{
			EventMode: "Resource",
			Resources: []APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Foo",
			}},
		},
		want: func() *apis.FieldError {
			var errs *apis.FieldError
			errs = errs.Also(apis.ErrGeneric("expected at least one, got none", "ref", "uri").ViaField("sink"))
			return errs
		}(),
	}, {
		name: "invalid mode",
		spec: ApiServerSourceSpec{
			EventMode: "Test",
			Resources: []APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Foo",
			}},
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
		want: func() *apis.FieldError {
			var errs *apis.FieldError
			errs = errs.Also(apis.ErrInvalidValue("Test", "mode"))
			return errs
		}(),
	}, {
		name: "invalid apiVersion",
		spec: ApiServerSourceSpec{
			EventMode: "Resource",
			Resources: []APIVersionKindSelector{{
				APIVersion: "v1/v2/v3",
				Kind:       "Foo",
			}},
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
		want: errors.New("invalid value: v1/v2/v3: resources[0].apiVersion"),
	}, {
		name: "missing kind",
		spec: ApiServerSourceSpec{
			EventMode: "Resource",
			Resources: []APIVersionKindSelector{{
				APIVersion: "v1",
			}},
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
		want: errors.New("missing field(s): resources[0].kind"),
	}, {
		name: "owner - invalid apiVersion",
		spec: ApiServerSourceSpec{
			EventMode: "Resource",
			Resources: []APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Bar",
			}},
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: "v1alpha1",
						Kind:       "broker",
						Name:       "default",
					},
				},
			},
			ResourceOwner: &APIVersionKind{
				APIVersion: "v1/v2/v3",
				Kind:       "Foo",
			},
		},
		want: errors.New("invalid value: v1/v2/v3: owner.apiVersion"),
	}, {
		name: "missing kind",
		spec: ApiServerSourceSpec{
			EventMode: "Resource",
			Resources: []APIVersionKindSelector{{
				APIVersion: "v1",
				Kind:       "Bar",
			}},
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: "v1alpha1",
						Kind:       "broker",
						Name:       "default",
					},
				},
			},
			ResourceOwner: &APIVersionKind{
				APIVersion: "v1",
			},
		},
		want: errors.New("missing field(s): owner.kind"),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.spec.Validate(context.TODO())
			if test.want != nil {
				if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
					t.Error("APIServerSourceSpec.Validate (-want, +got) =", diff)
				}
			} else if got != nil {
				t.Error("APIServerSourceSpec.Validate wanted nil, got =", got.Error())
			}
		})
	}
}
