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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	readySinkBinding = &SinkBinding{
		Status: SinkBindingStatus{
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

	notReadySinkBinding = &SinkBinding{
		Status: SinkBindingStatus{
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

func TestContainerSourceGetConditionSet(t *testing.T) {
	r := &ContainerSource{}

	if got, want := r.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func TestContainerSourceStatusIsReady(t *testing.T) {
	tests := []struct {
		name                string
		s                   *ContainerSourceStatus
		wantConditionStatus corev1.ConditionStatus
		want                bool
	}{{
		name: "uninitialized",
		s:    &ContainerSourceStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "mark ready ra",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateReceiveAdapterStatus(availableDeployment)
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "mark ready sb",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateSinkBindingStatus(&readySinkBinding.Status)
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "mark ready sb and ra",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateSinkBindingStatus(&readySinkBinding.Status)
			s.PropagateReceiveAdapterStatus(availableDeployment)
			return s
		}(),
		wantConditionStatus: corev1.ConditionTrue,
		want:                true,
	}, {
		name: "mark ready sb and unavailable ra",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateSinkBindingStatus(&readySinkBinding.Status)
			s.PropagateReceiveAdapterStatus(unavailableDeployment)
			return s
		}(),
		wantConditionStatus: corev1.ConditionFalse,
		want:                false,
	}, {
		name: "mark ready sb and unknown ra",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateSinkBindingStatus(&readySinkBinding.Status)
			s.PropagateReceiveAdapterStatus(unknownDeployment)
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "mark ready sb and not deployed ra",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateSinkBindingStatus(&readySinkBinding.Status)
			s.PropagateReceiveAdapterStatus(&appsv1.Deployment{})
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "mark ready sb and ra the no sb",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateSinkBindingStatus(&readySinkBinding.Status)
			s.PropagateReceiveAdapterStatus(availableDeployment)
			s.PropagateSinkBindingStatus(&notReadySinkBinding.Status)
			return s
		}(),
		wantConditionStatus: corev1.ConditionFalse,
		want:                false,
	}, {
		name: "mark ready sb and ra then not ra",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateReceiveAdapterStatus(availableDeployment)
			s.PropagateSinkBindingStatus(&notReadySinkBinding.Status)
			s.PropagateReceiveAdapterStatus(unavailableDeployment)
			return s
		}(),
		wantConditionStatus: corev1.ConditionFalse,
		want:                false,
	}, {
		name: "mark not ready sb and ready ra",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateSinkBindingStatus(&notReadySinkBinding.Status)
			s.PropagateReceiveAdapterStatus(availableDeployment)
			return s
		}(),
		wantConditionStatus: corev1.ConditionFalse,
		want:                false,
	}, {
		name: "mark not ready sb and ra then ready sb",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateSinkBindingStatus(&notReadySinkBinding.Status)
			s.PropagateReceiveAdapterStatus(availableDeployment)
			s.PropagateSinkBindingStatus(&readySinkBinding.Status)
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

func TestContainerSourceStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *ContainerSourceStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &ContainerSourceStatus{},
		condQuery: ContainerSourceConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: ContainerSourceConditionReady,
		want: &apis.Condition{
			Type:   ContainerSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark ready ra",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateReceiveAdapterStatus(availableDeployment)
			return s
		}(),
		condQuery: ContainerSourceConditionReady,
		want: &apis.Condition{
			Type:   ContainerSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark ready sb",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateSinkBindingStatus(&readySinkBinding.Status)
			return s
		}(),
		condQuery: ContainerSourceConditionReady,
		want: &apis.Condition{
			Type:   ContainerSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark ready sb and ra",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateSinkBindingStatus(&readySinkBinding.Status)
			s.PropagateReceiveAdapterStatus(availableDeployment)
			return s
		}(),
		condQuery: ContainerSourceConditionReady,
		want: &apis.Condition{
			Type:   ContainerSourceConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark ready sb and ra then no sb",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateSinkBindingStatus(&readySinkBinding.Status)
			s.PropagateReceiveAdapterStatus(availableDeployment)
			s.PropagateSinkBindingStatus(&notReadySinkBinding.Status)
			return s
		}(),
		condQuery: ContainerSourceConditionReady,
		want: &apis.Condition{
			Type:    ContainerSourceConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		},
	}, {
		name: "mark ready sb and ra then no ra",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateSinkBindingStatus(&readySinkBinding.Status)
			s.PropagateReceiveAdapterStatus(availableDeployment)
			s.PropagateReceiveAdapterStatus(unavailableDeployment)
			return s
		}(),
		condQuery: ContainerSourceConditionReady,
		want: &apis.Condition{
			Type:   ContainerSourceConditionReady,
			Status: corev1.ConditionFalse,
		},
	}, {
		name: "mark not ready sb and ready ra then ready sb",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateSinkBindingStatus(&notReadySinkBinding.Status)
			s.PropagateReceiveAdapterStatus(availableDeployment)
			s.PropagateSinkBindingStatus(&readySinkBinding.Status)
			return s
		}(),
		condQuery: ContainerSourceConditionReady,
		want: &apis.Condition{
			Type:   ContainerSourceConditionReady,
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
