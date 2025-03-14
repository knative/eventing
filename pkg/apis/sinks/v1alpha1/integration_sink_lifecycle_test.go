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
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestIntegrationSinkGetConditionSet(t *testing.T) {
	r := &IntegrationSink{}

	if got, want := r.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func TestIntegrationSinkInitializeConditions(t *testing.T) {
	tests := []struct {
		name string
		js   *IntegrationSinkStatus
		want *IntegrationSinkStatus
	}{{
		name: "empty",
		js:   &IntegrationSinkStatus{},
		want: &IntegrationSinkStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   IntegrationSinkConditionAddressable,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   IntegrationSinkConditionDeploymentReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   IntegrationSinkConditionEventPoliciesReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   IntegrationSinkConditionReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one false",
		js: &IntegrationSinkStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   IntegrationSinkConditionAddressable,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &IntegrationSinkStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   IntegrationSinkConditionAddressable,
					Status: corev1.ConditionFalse,
				}, {
					Type:   IntegrationSinkConditionDeploymentReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   IntegrationSinkConditionEventPoliciesReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   IntegrationSinkConditionReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one true",
		js: &IntegrationSinkStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   IntegrationSinkConditionAddressable,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		want: &IntegrationSinkStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   IntegrationSinkConditionAddressable,
					Status: corev1.ConditionTrue,
				}, {
					Type:   IntegrationSinkConditionDeploymentReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   IntegrationSinkConditionEventPoliciesReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   IntegrationSinkConditionReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.js.InitializeConditions()
			if diff := cmp.Diff(test.want, test.js, ignoreAllButTypeAndStatus); diff != "" {
				t.Error("unexpected conditions (-want, +got) =", diff)
			}
		})
	}
}
