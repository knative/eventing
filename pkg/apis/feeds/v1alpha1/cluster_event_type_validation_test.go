/*
Copyright 2018 The Knative Authors

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
	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
	"testing"
)

func TestClusterEventTypeSpecValidation(t *testing.T) {
	tests := []struct {
		name string
		et   *ClusterEventTypeSpec
		want *apis.FieldError
	}{{
		name: "valid",
		et: &ClusterEventTypeSpec{
			ClusterEventSource: "foo",
		},
	}, {
		name: "invalid source",
		et: &ClusterEventTypeSpec{
			ClusterEventSource: "f@o",
		},
		want: apis.ErrInvalidValue("f@o", "eventSource"),
	}, {
		name: "empty",
		et:   &ClusterEventTypeSpec{},
		want: apis.ErrMissingField("eventSource"),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.et.Validate()
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("validateClusterEventType (-want, +got) = %v", diff)
			}
		})
	}
}

func TestClusterEventTypeImmutableFields(t *testing.T) {
	tests := []struct {
		name string
		new  apis.Immutable
		old  apis.Immutable
		want *apis.FieldError
	}{{
		name: "good (no change)",
		new: &ClusterEventType{
			Spec: ClusterEventTypeSpec{
				ClusterEventSource: "foo",
			},
		},
		old: &ClusterEventType{
			Spec: ClusterEventTypeSpec{
				ClusterEventSource: "foo",
			},
		},
		want: nil,
	}, {
		name: "good (description change)",
		new: &ClusterEventType{
			Spec: ClusterEventTypeSpec{
				ClusterEventSource: "foo",
				CommonEventTypeSpec: CommonEventTypeSpec{
					Description: "Foo foo foo.",
				},
			},
		},
		old: &ClusterEventType{
			Spec: ClusterEventTypeSpec{
				ClusterEventSource: "foo",
				CommonEventTypeSpec: CommonEventTypeSpec{
					Description: "Bar bar bar.",
				},
			},
		},
		want: nil,
	}, {
		name: "bad (source changes)",
		new: &ClusterEventType{
			Spec: ClusterEventTypeSpec{
				ClusterEventSource: "foo",
			},
		},
		old: &ClusterEventType{
			Spec: ClusterEventTypeSpec{
				ClusterEventSource: "bar",
			},
		},
		want: &apis.FieldError{
			Message: "Immutable fields changed",
			Paths:   []string{"spec.eventSource"},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.new.CheckImmutableFields(test.old)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("Validate (-want, +got) = %v", diff)
			}
		})
	}
}
