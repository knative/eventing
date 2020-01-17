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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	corev1 "k8s.io/api/core/v1"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
)

var ignoreAllButTypeAndStatus = cmpopts.IgnoreFields(
	apis.Condition{},
	"LastTransitionTime", "Message", "Reason", "Severity")

var (
	configMapPropagationConditionReady = apis.Condition{
		Type:   ConfigMapPropagationConditionReady,
		Status: corev1.ConditionTrue,
	}

	configMapPropagationConditionPropagated = apis.Condition{
		Type:   ConfigMapPropagationConditionPropagated,
		Status: corev1.ConditionTrue,
	}
)

func TestConfigMapPropagationGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		cmps      *ConfigMapPropagationStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name: "single condition",
		cmps: &ConfigMapPropagationStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					configMapPropagationConditionReady,
				},
			},
		},
		condQuery: apis.ConditionReady,
		want:      &configMapPropagationConditionReady,
	}, {
		name: "multiple conditions",
		cmps: &ConfigMapPropagationStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					configMapPropagationConditionPropagated,
				},
			},
		},
		condQuery: ConfigMapPropagationConditionPropagated,
		want:      &configMapPropagationConditionPropagated,
	}, {
		name: "unknown condition",
		cmps: &ConfigMapPropagationStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					configMapPropagationConditionPropagated,
				},
			},
		},
		condQuery: apis.ConditionType("foo"),
		want:      nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.cmps.GetCondition(test.condQuery)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}

func TestConfigMapPropagationInitializeConditions(t *testing.T) {
	tests := []struct {
		name string
		cmps *ConfigMapPropagationStatus
		want *ConfigMapPropagationStatus
	}{{
		name: "empty",
		cmps: &ConfigMapPropagationStatus{},
		want: &ConfigMapPropagationStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   ConfigMapPropagationConditionPropagated,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   ConfigMapPropagationConditionReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one false",
		cmps: &ConfigMapPropagationStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   ConfigMapPropagationConditionPropagated,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &ConfigMapPropagationStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   ConfigMapPropagationConditionPropagated,
					Status: corev1.ConditionFalse,
				}, {
					Type:   ConfigMapPropagationConditionReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one true",
		cmps: &ConfigMapPropagationStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   ConfigMapPropagationConditionPropagated,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		want: &ConfigMapPropagationStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   ConfigMapPropagationConditionPropagated,
					Status: corev1.ConditionTrue,
				}, {
					Type:   ConfigMapPropagationConditionReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.cmps.InitializeConditions()
			if diff := cmp.Diff(test.want, test.cmps, ignoreAllButTypeAndStatus); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}

func TestConfigMapPropagationIsReady(t *testing.T) {
	tests := []struct {
		name            string
		markPropagation *bool
		wantReady       bool
	}{{
		name:            "all happy",
		markPropagation: ptr.Bool(true),
		wantReady:       true,
	}, {
		name:            "propagation sad",
		markPropagation: ptr.Bool(false),
		wantReady:       false,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmps := &ConfigMapPropagationStatus{}
			if test.markPropagation != nil {
				if *test.markPropagation {
					cmps.MarkPropagated()
				} else {
					cmps.MarkNotPropagated()
				}
			}
			if got := cmps.IsReady(); test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}
		})
	}
}

func TestSetCopyConfigMapStatus(t *testing.T) {
	tests := []struct {
		name  string
		cmpsc *ConfigMapPropagationStatusCopyConfigMap
		want  *ConfigMapPropagationStatusCopyConfigMap
	}{{
		name:  "all happy",
		cmpsc: &ConfigMapPropagationStatusCopyConfigMap{},
		want: &ConfigMapPropagationStatusCopyConfigMap{
			"name", "source", "operation", "ready", "reason", "resourceVersion",
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.cmpsc.SetCopyConfigMapStatus("name", "source", "operation", "ready", "reason", "resourceVersion")
			if got := test.cmpsc; !reflect.DeepEqual(test.want, got) {
				t.Errorf("unexpected readiness: want %v, got %v", test.want, got)
			}
		})
	}
}
