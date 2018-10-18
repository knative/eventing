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

var ignoreTransitionTimeMessageAndReason = cmpopts.IgnoreFields(duckv1alpha1.Condition{}, "LastTransitionTime", "Message", "Reason")

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
			}, {
				Type:   ChannelConditionSinkable,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   ChannelConditionSubscribable,
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
			}, {
				Type:   ChannelConditionSinkable,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   ChannelConditionSubscribable,
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
			}, {
				Type:   ChannelConditionSinkable,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   ChannelConditionSubscribable,
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
		setSubscribable bool
		setSinkable     bool
		wantReady       bool
	}{{
		name:            "all happy",
		markProvisioned: true,
		setSubscribable: true,
		setSinkable:     true,
		wantReady:       true,
	}, {
		name:            "one sad",
		markProvisioned: false,
		setSubscribable: true,
		setSinkable:     true,
		wantReady:       false,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cs := &ChannelStatus{}
			if test.markProvisioned {
				cs.MarkProvisioned()
			}
			if test.setSubscribable {
				cs.SetSubscribable("foo", "bar")
			}
			if test.setSinkable {
				cs.SetSinkable("foo.bar")
			}
			got := cs.IsReady()
			if test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}
		})
	}
}

func TestChannelStatus_SetSubscribable(t *testing.T) {
	testCases := map[string]struct {
		namespace string
		name      string
		want      *ChannelStatus
	}{
		"empty namespace and name": {
			want: &ChannelStatus{
				Conditions: []duckv1alpha1.Condition{
					// Note that Ready is here because when the condition is marked False, duck
					// automatically sets Ready to false.
					{
						Type:   ChannelConditionReady,
						Status: corev1.ConditionFalse,
					},
					{
						Type:   ChannelConditionSubscribable,
						Status: corev1.ConditionFalse,
					},
				},
			},
		},
		"empty namespace": {
			name: "foobar",
			want: &ChannelStatus{
				Subscribable: duckv1alpha1.Subscribable{
					Channelable: corev1.ObjectReference{
						APIVersion: SchemeGroupVersion.String(),
						Kind:       "Channel",
						Name:       "foobar",
					},
				},
				Conditions: []duckv1alpha1.Condition{
					{
						Type:   ChannelConditionSubscribable,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		"subscribable": {
			namespace: "test-namespace",
			name:      "test-name",
			want: &ChannelStatus{
				Subscribable: duckv1alpha1.Subscribable{
					Channelable: corev1.ObjectReference{
						APIVersion: SchemeGroupVersion.String(),
						Kind:       "Channel",
						Namespace:  "test-namespace",
						Name:       "test-name",
					},
				},
				Conditions: []duckv1alpha1.Condition{
					{
						Type:   ChannelConditionSubscribable,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			cs := &ChannelStatus{}
			cs.SetSubscribable(tc.namespace, tc.name)
			if diff := cmp.Diff(tc.want, cs, ignoreTransitionTimeMessageAndReason); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}

func TestChannelStatus_SetSinkable(t *testing.T) {
	testCases := map[string]struct {
		domainInternal string
		want           *ChannelStatus
	}{
		"empty string": {
			want: &ChannelStatus{
				Conditions: []duckv1alpha1.Condition{
					// Note that Ready is here because when the condition is marked False, duck
					// automatically sets Ready to false.
					{
						Type:   ChannelConditionReady,
						Status: corev1.ConditionFalse,
					},
					{
						Type:   ChannelConditionSinkable,
						Status: corev1.ConditionFalse,
					},
				},
			},
		},
		"has domain": {
			domainInternal: "test-domain",
			want: &ChannelStatus{
				Sinkable: duckv1alpha1.Sinkable{
					DomainInternal: "test-domain",
				},
				Conditions: []duckv1alpha1.Condition{
					{
						Type:   ChannelConditionSinkable,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			cs := &ChannelStatus{}
			cs.SetSinkable(tc.domainInternal)
			if diff := cmp.Diff(tc.want, cs, ignoreTransitionTimeMessageAndReason); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}
