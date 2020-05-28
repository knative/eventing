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

	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	availDeployment = &appsv1.Deployment{
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	unavailDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionFalse,
				},
			},
		},
	}

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
		name string
		s    *ContainerSourceStatus
		want bool
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
		want: false,
	}, {
		name: "mark ready ra",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateReceiveAdapterStatus(availDeployment)
			return s
		}(),
		want: false,
	}, {
		name: "mark ready sb",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateSinkBindingStatus(&readySinkBinding.Status)
			return s
		}(),
		want: false,
	}, {
		name: "mark ready sb and ra",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateSinkBindingStatus(&readySinkBinding.Status)
			s.PropagateReceiveAdapterStatus(availDeployment)
			return s
		}(),
		want: true,
	}, {
		name: "mark ready sb and ra the no sb",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateSinkBindingStatus(&readySinkBinding.Status)
			s.PropagateReceiveAdapterStatus(availDeployment)
			s.PropagateSinkBindingStatus(&notReadySinkBinding.Status)
			return s
		}(),
		want: false,
	}, {
		name: "mark ready sb and ra then not ra",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateReceiveAdapterStatus(availDeployment)
			s.PropagateSinkBindingStatus(&notReadySinkBinding.Status)
			s.PropagateReceiveAdapterStatus(unavailDeployment)
			return s
		}(),
		want: false,
	}, {
		name: "mark not ready sb and ready ra",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateSinkBindingStatus(&notReadySinkBinding.Status)
			s.PropagateReceiveAdapterStatus(availDeployment)
			return s
		}(),
		want: false,
	}, {
		name: "mark not ready sb and ra then ready sb",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateSinkBindingStatus(&notReadySinkBinding.Status)
			s.PropagateReceiveAdapterStatus(availDeployment)
			s.PropagateSinkBindingStatus(&readySinkBinding.Status)
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
			s.PropagateReceiveAdapterStatus(availDeployment)
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
			s.PropagateReceiveAdapterStatus(availDeployment)
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
			s.PropagateReceiveAdapterStatus(availDeployment)
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
			s.PropagateReceiveAdapterStatus(availDeployment)
			s.PropagateReceiveAdapterStatus(unavailDeployment)
			return s
		}(),
		condQuery: ContainerSourceConditionReady,
		want: &apis.Condition{
			Type:    ContainerSourceConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "DeploymentUnavailable",
			Message: "The Deployment 'test-name' is unavailable.",
		},
	}, {
		name: "mark not ready sb and ready ra then ready sb",
		s: func() *ContainerSourceStatus {
			s := &ContainerSourceStatus{}
			s.InitializeConditions()
			s.PropagateSinkBindingStatus(&notReadySinkBinding.Status)
			s.PropagateReceiveAdapterStatus(availDeployment)
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
