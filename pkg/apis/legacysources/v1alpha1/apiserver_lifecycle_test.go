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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

func TestApiServerSourceStatusIsReady(t *testing.T) {
	tests := []struct {
		name string
		s    *ApiServerSourceStatus
		want bool
	}{{
		name: "uninitialized",
		s:    &ApiServerSourceStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		want: false,
	}, {
		name: "mark deployed",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		want: false,
	}, {
		name: "mark sink",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			return s
		}(),
		want: false,
	}, {
		name: "mark sufficient permissions",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			s.MarkSufficientPermissions()
			return s
		}(),
		want: false,
	}, {
		name: "mark event types",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			s.MarkEventTypes()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink and sufficient permissions and deployed",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSufficientPermissions()
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		want: true,
	}, {
		name: "mark sink and sufficient permissions and deployed and event types",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSufficientPermissions()
			s.PropagateDeploymentAvailability(availableDeployment)
			s.MarkEventTypes()
			return s
		}(),
		want: true,
	}, {
		name: "mark sink and not enough permissions",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkNoSufficientPermissions("areason", "amessage")
			return s
		}(),
		want: false,
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

func TestApiServerSourceStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *ApiServerSourceStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &ApiServerSourceStatus{},
		condQuery: ApiServerConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: ApiServerConditionReady,
		want: &apis.Condition{
			Type:   ApiServerConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark deployed",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		condQuery: ApiServerConditionReady,
		want: &apis.Condition{
			Type:   ApiServerConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			return s
		}(),
		condQuery: ApiServerConditionReady,
		want: &apis.Condition{
			Type:   ApiServerConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink and enough permissions and deployed",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSufficientPermissions()
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		condQuery: ApiServerConditionReady,
		want: &apis.Condition{
			Type:   ApiServerConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark sink and enough permissions and deployed and event types",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSufficientPermissions()
			s.PropagateDeploymentAvailability(availableDeployment)
			s.MarkEventTypes()
			return s
		}(),
		condQuery: ApiServerConditionReady,
		want: &apis.Condition{
			Type:   ApiServerConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark sink empty and enough permissions and deployed and event types",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("")
			s.MarkSufficientPermissions()
			s.PropagateDeploymentAvailability(availableDeployment)
			s.MarkEventTypes()
			return s
		}(),
		condQuery: ApiServerConditionReady,
		want: &apis.Condition{
			Type:    ApiServerConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "SinkEmpty",
			Message: "Sink has resolved to empty.",
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
