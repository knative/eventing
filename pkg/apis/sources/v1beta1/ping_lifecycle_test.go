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
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"knative.dev/pkg/apis"
)

func TestPingSourceGetConditionSet(t *testing.T) {
	r := &PingSource{}

	if got, want := r.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func TestPingSource_GetGroupVersionKind(t *testing.T) {
	src := PingSource{}
	gvk := src.GetGroupVersionKind()

	if gvk.Kind != "PingSource" {
		t.Errorf("Should be PingSource.")
	}
}

func TestPingSource_PingSourceSource(t *testing.T) {
	cePingSource := PingSourceSource("ns1", "job1")

	if cePingSource != "/apis/v1/namespaces/ns1/pingsources/job1" {
		t.Errorf("Should be '/apis/v1/namespaces/ns1/pingsources/job1'")
	}
}

func TestPingSourceStatusIsReady(t *testing.T) {
	exampleUri, _ := apis.ParseURL("uri://example")

	tests := []struct {
		name                string
		s                   *PingSourceStatus
		wantConditionStatus corev1.ConditionStatus
		want                bool
	}{{
		name: "uninitialized",
		s:    &PingSourceStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *PingSourceStatus {
			s := &PingSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "mark deployed",
		s: func() *PingSourceStatus {
			s := &PingSourceStatus{}
			s.InitializeConditions()
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "mark sink",
		s: func() *PingSourceStatus {
			s := &PingSourceStatus{}
			s.InitializeConditions()

			s.MarkSink(exampleUri)
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "mark sink and deployed",
		s: func() *PingSourceStatus {
			s := &PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(exampleUri)
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		wantConditionStatus: corev1.ConditionTrue,
		want:                true,
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

func TestPingSourceStatusGetTopLevelCondition(t *testing.T) {
	exampleUri, _ := apis.ParseURL("uri://example")

	tests := []struct {
		name string
		s    *PingSourceStatus
		want *apis.Condition
	}{{
		name: "uninitialized",
		s:    &PingSourceStatus{},
		want: nil,
	}, {
		name: "initialized",
		s: func() *PingSourceStatus {
			s := &PingSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		want: &apis.Condition{
			Type:   PingSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark deployed",
		s: func() *PingSourceStatus {
			s := &PingSourceStatus{}
			s.InitializeConditions()
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		want: &apis.Condition{
			Type:   PingSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink",
		s: func() *PingSourceStatus {
			s := &PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(exampleUri)
			return s
		}(),
		want: &apis.Condition{
			Type:   PingSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink and deployed",
		s: func() *PingSourceStatus {
			s := &PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(exampleUri)
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		want: &apis.Condition{
			Type:   PingSourceConditionReady,
			Status: corev1.ConditionTrue,
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.s.GetTopLevelCondition()
			ignoreTime := cmpopts.IgnoreFields(apis.Condition{},
				"LastTransitionTime", "Severity")
			if diff := cmp.Diff(test.want, got, ignoreTime); diff != "" {
				t.Error("unexpected condition (-want, +got) =", diff)
			}
		})
	}
}

func TestPingSourceStatusGetCondition(t *testing.T) {
	exampleUri, _ := apis.ParseURL("uri://example")
	tests := []struct {
		name      string
		s         *PingSourceStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &PingSourceStatus{},
		condQuery: PingSourceConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *PingSourceStatus {
			s := &PingSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: PingSourceConditionReady,
		want: &apis.Condition{
			Type:   PingSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark deployed",
		s: func() *PingSourceStatus {
			s := &PingSourceStatus{}
			s.InitializeConditions()
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		condQuery: PingSourceConditionReady,
		want: &apis.Condition{
			Type:   PingSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink",
		s: func() *PingSourceStatus {
			s := &PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(exampleUri)
			return s
		}(),
		condQuery: PingSourceConditionReady,
		want: &apis.Condition{
			Type:   PingSourceConditionReady,
			Status: corev1.ConditionUnknown,
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
