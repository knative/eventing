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
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	readyContainerSource = &v1.ContainerSource{
		Status: v1.ContainerSourceStatus{
			SourceStatus: duckv1.SourceStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   apis.ConditionReady,
						Status: corev1.ConditionTrue,
					}},
				},
			},
		},
	}

	notReadyContainerSource = &v1.ContainerSource{
		Status: v1.ContainerSourceStatus{
			SourceStatus: duckv1.SourceStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:    apis.ConditionReady,
						Status:  corev1.ConditionFalse,
						Reason:  "Testing",
						Message: "hi",
					}},
				},
			},
		},
	}
)

func TestIntegrationSourceGetConditionSet(t *testing.T) {
	r := &IntegrationSource{}

	if got, want := r.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func TestIntegrationSourceStatusIsReady(t *testing.T) {
	tests := []struct {
		name                string
		s                   *IntegrationSourceStatus
		wantConditionStatus corev1.ConditionStatus
		want                bool
	}{{
		name: "uninitialized",
		s:    &IntegrationSourceStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *IntegrationSourceStatus {
			s := &IntegrationSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "mark ready container source",
		s: func() *IntegrationSourceStatus {
			s := &IntegrationSourceStatus{}
			s.InitializeConditions()
			s.PropagateContainerSourceStatus(&readyContainerSource.Status)
			return s
		}(),
		wantConditionStatus: corev1.ConditionTrue,
		want:                true,
	}, {
		name: "mark not ready container source",
		s: func() *IntegrationSourceStatus {
			s := &IntegrationSourceStatus{}
			s.InitializeConditions()
			s.PropagateContainerSourceStatus(&notReadyContainerSource.Status)
			return s
		}(),
		wantConditionStatus: corev1.ConditionFalse,
		want:                false,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.wantConditionStatus != "" {
				gotConditionStatus := test.s.GetTopLevelCondition().Status
				if gotConditionStatus != test.wantConditionStatus {
					t.Errorf("unexpected condition status: want %v, got %v", test.wantConditionStatus, gotConditionStatus)
				}
			}
			got := test.s.IsReady()
			if got != test.want {
				t.Errorf("unexpected readiness: want %v, got %v", test.want, got)
			}
		})
	}
}

func TestIntegrationSourceStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *IntegrationSourceStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &IntegrationSourceStatus{},
		condQuery: IntegrationSourceConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *IntegrationSourceStatus {
			s := &IntegrationSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: IntegrationSourceConditionReady,
		want: &apis.Condition{
			Type:   IntegrationSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark ready cs",
		s: func() *IntegrationSourceStatus {
			s := &IntegrationSourceStatus{}
			s.InitializeConditions()
			s.PropagateContainerSourceStatus(&readyContainerSource.Status)
			return s
		}(),
		condQuery: IntegrationSourceConditionReady,
		want: &apis.Condition{
			Type:   IntegrationSourceConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark ready cs then no cs",
		s: func() *IntegrationSourceStatus {
			s := &IntegrationSourceStatus{}
			s.InitializeConditions()
			s.PropagateContainerSourceStatus(&readyContainerSource.Status)
			s.PropagateContainerSourceStatus(&notReadyContainerSource.Status)
			return s
		}(),
		condQuery: IntegrationSourceConditionReady,
		want: &apis.Condition{
			Type:    IntegrationSourceConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.s.GetCondition(test.condQuery)
			ignoreTime := cmpopts.IgnoreFields(apis.Condition{},
				"LastTransitionTime", "Severity")
			if diff := cmp.Diff(test.want, got, ignoreTime); diff != "" {
				t.Error("unexpected condition (-want, +got) =", diff)
			}
		})
	}
}
