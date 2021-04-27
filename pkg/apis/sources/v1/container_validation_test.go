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
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestContainerSourceValidation(t *testing.T) {
	tests := []struct {
		name string
		spec ContainerSourceSpec
		want *apis.FieldError
	}{{
		name: "missing container",
		spec: ContainerSourceSpec{
			Template: corev1.PodTemplateSpec{},
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: "v1",
						Kind:       "Broker",
						Name:       "default",
					},
				},
			},
		},
		want: func() *apis.FieldError {
			var errs *apis.FieldError
			fe := apis.ErrMissingField("containers")
			errs = errs.Also(fe)
			return errs
		}(),
	}, {
		name: "missing container image",
		spec: ContainerSourceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "test-name",
					}},
				},
			},
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: "eventing.knative.dev/v1",
						Kind:       "Broker",
						Name:       "default",
					},
				},
			},
		},
		want: func() *apis.FieldError {
			var errs *apis.FieldError
			fe := apis.ErrMissingField("containers[0].image")
			errs = errs.Also(fe)
			return errs
		}(),
	}, {
		name: "empty sink",
		spec: ContainerSourceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "name",
						Image: "image",
					}},
				},
			},
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{},
			},
		},
		want: func() *apis.FieldError {
			var errs *apis.FieldError
			fe := apis.ErrGeneric("expected at least one, got none", "sink.ref", "sink.uri")
			errs = errs.Also(fe)
			return errs
		}(),
	},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.spec.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Error("ContainerSourceSpec.Validate (-want, +got) =", diff)
			}
		})
	}
}
