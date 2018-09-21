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
	"k8s.io/api/core/v1"
	"testing"
)

func TestFlowSpecValidation(t *testing.T) {
	tests := []struct {
		name string
		f    *FlowSpec
		want *apis.FieldError
	}{{
		name: "valid with URI",
		f: &FlowSpec{
			Action: FlowAction{
				TargetURI: stringPtr("http://example.com/action"),
			},
			Trigger: EventTrigger{
				EventType: "foo",
				Resource:  "bar",
			},
		},
	}, {
		name: "valid with Target",
		f: &FlowSpec{
			Action: FlowAction{
				Target: &v1.ObjectReference{
					Name: "foo",
				},
			},
			Trigger: EventTrigger{
				EventType: "foo",
				Resource:  "bar",
			},
		},
	}, {
		name: "invalid with URI",
		f: &FlowSpec{
			Action: FlowAction{
				TargetURI: stringPtr("http//example.com/action"),
			},
			Trigger: EventTrigger{
				EventType: "foo",
				Resource:  "bar",
			},
		},
		want: &apis.FieldError{
			Message: `invalid value "http//example.com/action"`,
			Paths:   []string{"action.targetURI"},
		},
	}, {
		name: "invalid with Target",
		f: &FlowSpec{
			Action: FlowAction{
				Target: &v1.ObjectReference{
					Name: "f@o",
				},
			},
			Trigger: EventTrigger{
				EventType: "foo",
				Resource:  "bar",
			},
		},
		want: &apis.FieldError{
			Message: `invalid value "f@o"`,
			Paths:   []string{"action.target.name"},
		},
	}, {
		name: "invalid with both target and targetURI",
		f: &FlowSpec{
			Action: FlowAction{
				TargetURI: stringPtr("http://example.com/action"),
				Target: &v1.ObjectReference{
					Name: "foo",
				},
			},
			Trigger: EventTrigger{
				EventType: "foo",
				Resource:  "bar",
			},
		},
		want: &apis.FieldError{
			Message: `expected exactly one, got both`,
			Paths:   []string{"action.target", "action.targetURI"},
		},
	}, {
		name: "invalid, no target or targetURI",
		f: &FlowSpec{
			Action: FlowAction{},
			Trigger: EventTrigger{
				EventType: "foo",
				Resource:  "bar",
			},
		},
		want: &apis.FieldError{
			Message: `expected exactly one, got neither`,
			Paths:   []string{"action.target", "action.targetURI"},
		},
	}, {
		name: "invalid, missing event type",
		f: &FlowSpec{
			Action: FlowAction{
				TargetURI: stringPtr("http://example.com/action"),
			},
			Trigger: EventTrigger{
				Resource: "bar",
			},
		},
		want: &apis.FieldError{
			Message: "missing field(s)",
			Paths:   []string{"trigger.eventType"},
		},
	}, {
		name: "invalid event type",
		f: &FlowSpec{
			Action: FlowAction{
				TargetURI: stringPtr("http://example.com/action"),
			},
			Trigger: EventTrigger{
				EventType: "f@o",
				Resource:  "bar",
			},
		},
		want: &apis.FieldError{
			Message: `invalid value "f@o"`,
			Paths:   []string{"trigger.eventType"},
		},
	}, {
		name: "invalid, missing resource",
		f: &FlowSpec{
			Action: FlowAction{
				TargetURI: stringPtr("http://example.com/action"),
			},
			Trigger: EventTrigger{
				EventType: "foo",
			},
		},
		want: &apis.FieldError{
			Message: "missing field(s)",
			Paths:   []string{"trigger.resource"},
		},
	}, {
		name: "empty",
		f:    &FlowSpec{},
		want: &apis.FieldError{
			Message: "missing field(s)",
			Paths:   []string{"trigger", "action"},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.f.Validate()
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("validateFlow (-want, +got) = %v", diff)
			}
		})
	}
}

// stringPtr returns a pointer to the passed string.
func stringPtr(s string) *string {
	return &s
}
