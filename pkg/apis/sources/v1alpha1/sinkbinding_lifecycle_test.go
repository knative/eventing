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
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/tracker"
)

func TestSinkBindingGetGroupVersionKind(t *testing.T) {
	r := &SinkBinding{}
	want := schema.GroupVersionKind{
		Group:   "sources.eventing.knative.dev",
		Version: "v1alpha1",
		Kind:    "SinkBinding",
	}
	if got := r.GetGroupVersionKind(); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestSinkBindingGetUntypedSpec(t *testing.T) {
	r := &SinkBinding{
		Spec: SinkBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					APIVersion: "foo",
				},
			},
		},
	}
	want := r.Spec
	if got := r.GetUntypedSpec(); !reflect.DeepEqual(got, want) {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestSinkBindingStatusIsReady(t *testing.T) {
	tests := []struct {
		name string
		s    *SinkBindingStatus
		want bool
	}{{
		name: "uninitialized",
		s:    &SinkBindingStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *SinkBindingStatus {
			s := &SinkBindingStatus{}
			s.InitializeConditions()
			return s
		}(),
		want: false,
	}, {
		name: "mark available",
		s: func() *SinkBindingStatus {
			s := &SinkBindingStatus{}
			s.InitializeConditions()
			s.MarkBindingUnavailable("TheReason", "this is the message")
			return s
		}(),
		want: false,
	}, {
		name: "mark available",
		s: func() *SinkBindingStatus {
			s := &SinkBindingStatus{}
			s.InitializeConditions()
			s.MarkBindingAvailable()
			return s
		}(),
		want: true,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.s.IsReady()
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("%s: unexpected condition (-want, +got) = %v", test.name, diff)
			}
		})
	}
}
