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

package v1alpha1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

func TestContainerSourceValidation(t *testing.T) {
	tests := []struct {
		name string
		spec ContainerSourceSpec
		want *apis.FieldError
	}{{
		name: "valid spec",
		spec: ContainerSourceSpec{
			Template: &corev1.PodTemplateSpec{},
			Sink: &duckv1beta1.Destination{
				Ref: &corev1.ObjectReference{
					APIVersion: "v1alpha1",
					Kind:       "broker",
					Name:       "default",
				},
			},
		},
		want: nil,
	}, {
		name: "empty sink",
		spec: ContainerSourceSpec{
			Template: &corev1.PodTemplateSpec{},
		},
		want: func() *apis.FieldError {
			var errs *apis.FieldError
			fe := apis.ErrMissingField("sink")
			errs = errs.Also(fe)
			return errs
		}(),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.spec.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("ContainerSourceSpec.Validate (-want, +got) = %v", diff)
			}
		})
	}
}
