/*
Copyright 2018 The Knative Authors

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
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var condReady = duckv1alpha1.Condition{
	Type:   ChannelConditionReady,
	Status: corev1.ConditionTrue,
}

var condUnprovisioned = duckv1alpha1.Condition{
	Type:   ChannelConditionProvisioned,
	Status: corev1.ConditionFalse,
}

func TestChannelGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		cs        *ChannelStatus
		condQuery duckv1alpha1.ConditionType
		want      *duckv1alpha1.Condition
	}{{
		name: "single condition",
		cs: &ChannelStatus{
			Conditions: []duckv1alpha1.Condition{
				condReady,
			},
		},
		condQuery: duckv1alpha1.ConditionReady,
		want:      &condReady,
	}, {
		name: "multiple conditions",
		cs: &ChannelStatus{
			Conditions: []duckv1alpha1.Condition{
				condReady,
				condUnprovisioned,
			},
		},
		condQuery: ChannelConditionProvisioned,
		want:      &condUnprovisioned,
	}, {
		name: "unknown condition",
		cs: &ChannelStatus{
			Conditions: []duckv1alpha1.Condition{
				condReady,
				condUnprovisioned,
			},
		},
		condQuery: duckv1alpha1.ConditionType("foo"),
		want:      nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.cs.GetCondition(test.condQuery)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}

func TestChannelInitializeConditions(t *testing.T) {
	tests := []struct {
		name string
		cs   *ChannelStatus
		want *ChannelStatus
	}{{
		name: "empty",
		cs:   &ChannelStatus{},
		want: &ChannelStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   ChannelConditionProvisioned,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   ChannelConditionReady,
				Status: corev1.ConditionUnknown,
			}},
		},
	}, {
		name: "one false",
		cs: &ChannelStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   ChannelConditionProvisioned,
				Status: corev1.ConditionFalse,
			}},
		},
		want: &ChannelStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   ChannelConditionProvisioned,
				Status: corev1.ConditionFalse,
			}, {
				Type:   ChannelConditionReady,
				Status: corev1.ConditionUnknown,
			}},
		},
	}, {
		name: "one true",
		cs: &ChannelStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   ChannelConditionProvisioned,
				Status: corev1.ConditionTrue,
			}},
		},
		want: &ChannelStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   ChannelConditionProvisioned,
				Status: corev1.ConditionTrue,
			}, {
				Type:   ChannelConditionReady,
				Status: corev1.ConditionUnknown,
			}}},
	},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.cs.InitializeConditions()
			ignore := cmpopts.IgnoreFields(duckv1alpha1.Condition{}, "LastTransitionTime")
			if diff := cmp.Diff(test.want, test.cs, ignore); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}

func TestChannelIsReady(t *testing.T) {
	tests := []struct {
		name            string
		markProvisioned bool
		wantReady       bool
	}{{
		name:            "all happy",
		markProvisioned: true,
		wantReady:       true,
	}, {
		name:            "one sad",
		markProvisioned: false,
		wantReady:       false,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cs := &ChannelStatus{}
			if test.markProvisioned {
				cs.MarkProvisioned()
			}
			got := cs.IsReady()
			if test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}
		})
	}
}
