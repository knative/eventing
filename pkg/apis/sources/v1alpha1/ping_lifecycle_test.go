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

package v1alpha1_test

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/pkg/apis"
)

var (
	availableDeployment = &appsv1.Deployment{
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
)

func TestPingSource_GetGroupVersionKind(t *testing.T) {
	src := v1alpha1.PingSource{}
	gvk := src.GetGroupVersionKind()

	if gvk.Kind != "PingSource" {
		t.Errorf("Should be PingSource.")
	}
}

func TestPingSource_PingSourceSource(t *testing.T) {
	cePingSource := v1alpha1.PingSourceSource("ns1", "job1")

	if cePingSource != "/apis/v1/namespaces/ns1/pingsources/job1" {
		t.Errorf("Should be '/apis/v1/namespaces/ns1/pingsources/job1'")
	}
}

func TestPingSourceStatusIsReady(t *testing.T) {
	tests := []struct {
		name string
		s    *v1alpha1.PingSourceStatus
		want bool
	}{{
		name: "uninitialized",
		s:    &v1alpha1.PingSourceStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		want: false,
	}, {
		name: "mark deployed",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		want: false,
	}, {
		name: "mark sink",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			return s
		}(),
		want: false,
	}, {
		name: "mark schedule",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			return s
		}(),
		want: false,
	}, {
		name: "mark event types",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkEventType()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink and deployed",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		want: false,
	}, {
		name: "mark schedule and sink",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("uri://example")
			return s
		}(),
		want: false,
	}, {
		name: "mark schedule, sink and deployed",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("uri://example")
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		want: true,
	}, {
		name: "mark schedule, sink, deployed, and event types",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("uri://example")
			s.PropagateDeploymentAvailability(availableDeployment)
			s.MarkEventType()
			return s
		}(),
		want: true,
	}, {
		name: "mark schedule, sink and deployed then not deployed",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("uri://example")
			s.PropagateDeploymentAvailability(availableDeployment)
			s.PropagateDeploymentAvailability(&appsv1.Deployment{})
			return s
		}(),
		want: false,
	}, {
		name: "mark schedule, sink, deployed and event types then no event types",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("uri://example")
			s.PropagateDeploymentAvailability(availableDeployment)
			s.MarkNoEventType("Testing", "")
			return s
		}(),
		want: true,
	}, {
		name: "mark schedule validated, sink empty and deployed",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("")
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		want: false,
	}, {
		name: "mark schedule validated, sink empty and deployed then sink",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("")
			s.PropagateDeploymentAvailability(availableDeployment)
			s.MarkSink("uri://example")
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

func TestPingSourceStatusGetTopLevelCondition(t *testing.T) {
	tests := []struct {
		name string
		s    *v1alpha1.PingSourceStatus
		want *apis.Condition
	}{{
		name: "uninitialized",
		s:    &v1alpha1.PingSourceStatus{},
		want: nil,
	}, {
		name: "initialized",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		want: &apis.Condition{
			Type:   v1alpha1.PingSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark deployed",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		want: &apis.Condition{
			Type:   v1alpha1.PingSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			return s
		}(),
		want: &apis.Condition{
			Type:   v1alpha1.PingSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark schedule",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			return s
		}(),
		want: &apis.Condition{
			Type:   v1alpha1.PingSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark event types",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkEventType()
			return s
		}(),
		want: &apis.Condition{
			Type:   v1alpha1.PingSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink and deployed",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		want: &apis.Condition{
			Type:   v1alpha1.PingSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark schedule and sink",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("uri://example")
			return s
		}(),
		want: &apis.Condition{
			Type:   v1alpha1.PingSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark schedule, sink and deployed",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("uri://example")
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		want: &apis.Condition{
			Type:   v1alpha1.PingSourceConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark schedule, sink, deployed, and event types",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("uri://example")
			s.PropagateDeploymentAvailability(availableDeployment)
			s.MarkEventType()
			return s
		}(),
		want: &apis.Condition{
			Type:   v1alpha1.PingSourceConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark schedule, sink and deployed then not deployed",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("uri://example")
			s.PropagateDeploymentAvailability(availableDeployment)
			s.PropagateDeploymentAvailability(&appsv1.Deployment{})
			return s
		}(),
		want: &apis.Condition{
			Type:    v1alpha1.PingSourceConditionReady,
			Reason:  "DeploymentUnavailable",
			Status:  corev1.ConditionFalse,
			Message: "The Deployment '' is unavailable.",
		},
	}, {
		name: "mark schedule, sink, deployed and event types then no event types",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("uri://example")
			s.PropagateDeploymentAvailability(availableDeployment)
			s.MarkNoEventType("Testing", "")
			return s
		}(),
		want: &apis.Condition{
			Type:   v1alpha1.PingSourceConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark schedule validated, sink empty and deployed",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("")
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		want: &apis.Condition{
			Type:    v1alpha1.PingSourceConditionReady,
			Reason:  "SinkEmpty",
			Status:  corev1.ConditionFalse,
			Message: "Sink has resolved to empty.",
		},
	}, {
		name: "mark schedule validated, sink empty and deployed then sink",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("")
			s.PropagateDeploymentAvailability(availableDeployment)
			s.MarkSink("uri://example")
			return s
		}(),
		want: &apis.Condition{
			Type:   v1alpha1.PingSourceConditionReady,
			Status: corev1.ConditionTrue,
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.s.GetTopLevelCondition()
			ignoreTime := cmpopts.IgnoreFields(apis.Condition{},
				"LastTransitionTime", "Severity")
			if diff := cmp.Diff(test.want, got, ignoreTime); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}

func TestPingSourceStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *v1alpha1.PingSourceStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &v1alpha1.PingSourceStatus{},
		condQuery: v1alpha1.PingSourceConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: v1alpha1.PingSourceConditionReady,
		want: &apis.Condition{
			Type:   v1alpha1.PingSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark deployed",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		condQuery: v1alpha1.PingSourceConditionReady,
		want: &apis.Condition{
			Type:   v1alpha1.PingSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			return s
		}(),
		condQuery: v1alpha1.PingSourceConditionReady,
		want: &apis.Condition{
			Type:   v1alpha1.PingSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark schedule",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			return s
		}(),
		condQuery: v1alpha1.PingSourceConditionReady,
		want: &apis.Condition{
			Type:   v1alpha1.PingSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark schedule, sink and deployed",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("uri://example")
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		condQuery: v1alpha1.PingSourceConditionReady,
		want: &apis.Condition{
			Type:   v1alpha1.PingSourceConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark schedule, sink, deployed, and event types",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("uri://example")
			s.PropagateDeploymentAvailability(availableDeployment)
			s.MarkEventType()
			return s
		}(),
		condQuery: v1alpha1.PingSourceConditionReady,
		want: &apis.Condition{
			Type:   v1alpha1.PingSourceConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark schedule, sink and deployed then no sink",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("uri://example")
			s.PropagateDeploymentAvailability(availableDeployment)
			s.MarkNoSink("Testing", "hi%s", "")
			return s
		}(),
		condQuery: v1alpha1.PingSourceConditionReady,
		want: &apis.Condition{
			Type:    v1alpha1.PingSourceConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		},
	}, {
		name: "mark schedule, sink and deployed then invalid schedule",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("uri://example")
			s.PropagateDeploymentAvailability(availableDeployment)
			s.MarkInvalidSchedule("Testing", "hi%s", "")
			return s
		}(),
		condQuery: v1alpha1.PingSourceConditionReady,
		want: &apis.Condition{
			Type:    v1alpha1.PingSourceConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		},
	}, {
		name: "mark schedule, sink and deployed then deploying",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("uri://example")
			s.PropagateDeploymentAvailability(availableDeployment)
			s.PropagateDeploymentAvailability(&appsv1.Deployment{})
			return s
		}(),
		condQuery: v1alpha1.PingSourceConditionReady,
		want: &apis.Condition{
			Type:    v1alpha1.PingSourceConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "DeploymentUnavailable",
			Message: "The Deployment '' is unavailable.",
		},
	}, {
		name: "mark schedule, sink and deployed then not deployed",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("uri://example")
			s.PropagateDeploymentAvailability(availableDeployment)
			s.PropagateDeploymentAvailability(&appsv1.Deployment{})
			return s
		}(),
		condQuery: v1alpha1.PingSourceConditionReady,
		want: &apis.Condition{
			Type:    v1alpha1.PingSourceConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "DeploymentUnavailable",
			Message: "The Deployment '' is unavailable.",
		},
	}, {
		name: "mark schedule, sink, deployed and event types, then no event types",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("uri://example")
			s.PropagateDeploymentAvailability(availableDeployment)
			s.MarkEventType()
			s.MarkNoEventType("Testing", "hi")
			return s
		}(),
		condQuery: v1alpha1.PingSourceConditionReady,
		want: &apis.Condition{
			Type:   v1alpha1.PingSourceConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark schedule, sink empty and deployed",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("")
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		condQuery: v1alpha1.PingSourceConditionReady,
		want: &apis.Condition{
			Type:    v1alpha1.PingSourceConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "SinkEmpty",
			Message: "Sink has resolved to empty.",
		},
	}, {
		name: "mark schedule, sink empty and deployed then sink",
		s: func() *v1alpha1.PingSourceStatus {
			s := &v1alpha1.PingSourceStatus{}
			s.InitializeConditions()
			s.MarkSchedule()
			s.MarkSink("")
			s.PropagateDeploymentAvailability(availableDeployment)
			s.MarkSink("uri://example")
			return s
		}(),
		condQuery: v1alpha1.PingSourceConditionReady,
		want: &apis.Condition{
			Type:   v1alpha1.PingSourceConditionReady,
			Status: corev1.ConditionTrue,
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.s.GetCondition(test.condQuery)
			ignoreTime := cmpopts.IgnoreFields(apis.Condition{},
				"LastTransitionTime", "Severity")
			if diff := cmp.Diff(test.want, got, ignoreTime); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}
