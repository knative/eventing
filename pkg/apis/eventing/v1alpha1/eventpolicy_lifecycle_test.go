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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	eventPolicyConditionReady = apis.Condition{
		Type:   EventPolicyConditionReady,
		Status: corev1.ConditionTrue,
	}

	ignoreAllButTypeAndStatus = cmpopts.IgnoreFields(
		apis.Condition{},
		"LastTransitionTime", "Message", "Reason", "Severity")
)

func TestEventPolicyGetConditionSet(t *testing.T) {
	r := &EventPolicy{}

	if got, want := r.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func TestEventPolicyGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		ets       *EventPolicyStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name: "single condition",
		ets: &EventPolicyStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					eventPolicyConditionReady,
				},
			},
		},
		condQuery: apis.ConditionReady,
		want:      &eventPolicyConditionReady,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ets.GetCondition(test.condQuery)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Error("unexpected condition (-want, +got) =", diff)
			}
		})
	}
}

func TestEventPolicyInitializeConditions(t *testing.T) {
	tests := []struct {
		name string
		ets  *EventPolicyStatus
		want *EventPolicyStatus
	}{
		{
			name: "empty",
			ets:  &EventPolicyStatus{},
			want: &EventPolicyStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   EventPolicyConditionReady,
						Status: corev1.ConditionUnknown,
					},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.ets.InitializeConditions()
			if diff := cmp.Diff(test.want, test.ets, ignoreAllButTypeAndStatus); diff != "" {
				t.Error("unexpected conditions (-want, +got) =", diff)
			}
		})
	}
}
