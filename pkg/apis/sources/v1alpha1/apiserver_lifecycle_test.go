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
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

var (
	availableDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "available",
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	unavailableDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "unavailable",
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

	unknownDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "unknown",
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionUnknown,
				},
			},
		},
	}
)

func TestApiServerSourceGetGroupVersionKind(t *testing.T) {
	r := &ApiServerSource{}
	want := schema.GroupVersionKind{
		Group:   "sources.knative.dev",
		Version: "v1alpha1",
		Kind:    "ApiServerSource",
	}
	if got := r.GetGroupVersionKind(); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestApiServerSourceStatusIsReady(t *testing.T) {
	tests := []struct {
		name                string
		s                   *ApiServerSourceStatus
		wantConditionStatus corev1.ConditionStatus
		want                bool
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
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "mark deployed",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "mark sink",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "mark sufficient permissions",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			s.MarkSufficientPermissions()
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
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
		wantConditionStatus: corev1.ConditionTrue,
		want:                true,
	}, {
		name: "mark sink and sufficient permissions and unavailable deployment",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSufficientPermissions()
			s.PropagateDeploymentAvailability(unavailableDeployment)
			return s
		}(),
		wantConditionStatus: corev1.ConditionFalse,
		want:                false,
	}, {
		name: "mark sink and sufficient permissions and unknown deployment",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSufficientPermissions()
			s.PropagateDeploymentAvailability(unknownDeployment)
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "mark sink and sufficient permissions and not deployed",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSufficientPermissions()
			s.PropagateDeploymentAvailability(&appsv1.Deployment{})
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "mark sink and not enough permissions",
		s: func() *ApiServerSourceStatus {
			s := &ApiServerSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkNoSufficientPermissions("areason", "amessage")
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

func TestApiServerSourceGetters(t *testing.T) {
	r := &ApiServerSource{
		Spec: ApiServerSourceSpec{
			ServiceAccountName: "test",
			Mode:               "test",
		},
	}
	if got, want := r.GetUntypedSpec(), r.Spec; !reflect.DeepEqual(got, want) {
		t.Errorf("GetUntypedSpec() = %v, want: %v", got, want)
	}
}
