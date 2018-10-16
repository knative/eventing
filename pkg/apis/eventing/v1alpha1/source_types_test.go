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
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func TestSourceStatusIsReady(t *testing.T) {
	tests := []struct {
		name string
		s    *SourceStatus
		want bool
	}{{
		name: "uninitialized",
		s:    &SourceStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *SourceStatus {
			s := &SourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		want: false,
	}, {
		name: "ready true condition",
		s: &SourceStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   SourceConditionReady,
				Status: corev1.ConditionTrue,
			}, {
				Type:   SourceConditionProvisioned,
				Status: corev1.ConditionTrue,
			}},
		},
		want: true,
	}, {
		name: "ready false condition",
		s: &SourceStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   SourceConditionReady,
				Status: corev1.ConditionFalse,
			}, {
				Type:   SourceConditionProvisioned,
				Status: corev1.ConditionTrue,
			}},
		},
		want: false,
	}, {
		name: "unknown condition",
		s: &SourceStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   "foo",
				Status: corev1.ConditionTrue,
			}},
		},
		want: false,
	}, {
		name: "mark provisioned",
		s: func() *SourceStatus {
			s := &SourceStatus{}
			s.InitializeConditions()
			s.MarkProvisioned()
			return s
		}(),
		want: true,
	}, {
		name: "mark deprovisioned",
		s: func() *SourceStatus {
			s := &SourceStatus{}
			s.InitializeConditions()
			s.MarkDeprovisioned("Testing", "Just a test")
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

func TestSourceStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *SourceStatus
		condQuery duckv1alpha1.ConditionType
		want      *duckv1alpha1.Condition
	}{{
		name: "single condition",
		s: &SourceStatus{
			Conditions: []duckv1alpha1.Condition{
				condReady,
			},
		},
		condQuery: duckv1alpha1.ConditionReady,
		want:      &condReady,
	}, {
		name: "unknown condition",
		s: &SourceStatus{
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
			got := test.s.GetCondition(test.condQuery)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}

func TestSourceStatusSetProvisionedObjectState(t *testing.T) {
	tests := []struct {
		name   string
		s      *SourceStatus
		update ProvisionedObjectStatus
		extra  []interface{}
		want   []ProvisionedObjectStatus
	}{{
		name: "empty to one",
		s:    &SourceStatus{},
		update: ProvisionedObjectStatus{
			Ref: corev1.ObjectReference{
				Name: "t",
			},
			Status: "Test",
			Reason: "a test",
		},
		want: []ProvisionedObjectStatus{{
			Ref: corev1.ObjectReference{
				Name: "t",
			},
			Status: "Test",
			Reason: "a test",
		}},
	}, {
		name: "update one to one",
		s: &SourceStatus{
			Provisioned: []ProvisionedObjectStatus{{
				Ref: corev1.ObjectReference{
					Name: "t",
				},
			}},
		},
		update: ProvisionedObjectStatus{
			Ref: corev1.ObjectReference{
				Name: "t",
			},
			Status: "Test",
			Reason: "a test",
		},
		want: []ProvisionedObjectStatus{{
			Ref: corev1.ObjectReference{
				Name: "t",
			},
			Status: "Test",
			Reason: "a test",
		}},
	}, {
		name: "update one with many",
		s: &SourceStatus{
			Provisioned: []ProvisionedObjectStatus{{
				Ref: corev1.ObjectReference{
					Name: "t",
					Kind: "OneKind",
				},
			}, {
				Ref: corev1.ObjectReference{
					Name: "t2",
				},
			}, {
				Ref: corev1.ObjectReference{
					Name: "t",
					Kind: "AnotherKind",
				},
			}},
		},
		update: ProvisionedObjectStatus{
			Ref: corev1.ObjectReference{
				Name: "t",
				Kind: "OneKind",
			},
			Status: "Test",
			Reason: "a test",
		},
		want: []ProvisionedObjectStatus{{
			Ref: corev1.ObjectReference{
				Name: "t",
				Kind: "OneKind",
			},
			Status: "Test",
			Reason: "a test",
		}, {
			Ref: corev1.ObjectReference{
				Name: "t2",
			},
		}, {
			Ref: corev1.ObjectReference{
				Name: "t",
				Kind: "AnotherKind",
			},
		}},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			u := test.update
			test.s.SetProvisionedObjectState(u.Ref, u.Status, u.Reason, test.extra...)

			got := test.s.Provisioned
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("%s: unexpected (-want, +got) = %v", test.name, diff)
			}
		})
	}
}
