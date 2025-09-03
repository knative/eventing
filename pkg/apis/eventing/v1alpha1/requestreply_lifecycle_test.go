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
	"k8s.io/utils/ptr"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	requestReplyConditionReady = apis.Condition{
		Type:   RequestReplyConditionReady,
		Status: corev1.ConditionTrue,
	}
)

func TestRequestReplyGetConditionSet(t *testing.T) {
	r := &RequestReply{}

	if got, want := r.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func TestRequestReplyGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		rrs       *RequestReplyStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name: "single condition",
		rrs: &RequestReplyStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					requestReplyConditionReady,
				},
			},
		},
		condQuery: apis.ConditionReady,
		want:      &eventPolicyConditionReady,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.rrs.GetCondition(test.condQuery)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Error("unexpected condition (-want, +got) =", diff)
			}
		})
	}
}

func TestRequestReplyInitializeConditions(t *testing.T) {
	tests := []struct {
		name string
		rrs  *RequestReplyStatus
		want *RequestReplyStatus
	}{
		{
			name: "empty status",
			rrs:  &RequestReplyStatus{},
			want: &RequestReplyStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						{
							Type:   RequestReplyConditionAddressable,
							Status: corev1.ConditionUnknown,
						},
						{
							Type:   RequestreplyConditionBrokerReady,
							Status: corev1.ConditionUnknown,
						},
						{
							Type:   RequestReplyConditionEventPoliciesReady,
							Status: corev1.ConditionUnknown,
						},
						{
							Type:   RequestReplyConditionReady,
							Status: corev1.ConditionUnknown,
						},
						{
							Type:   RequestReplyConditionTriggers,
							Status: corev1.ConditionUnknown,
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.rrs.InitializeConditions()
			if diff := cmp.Diff(test.want, test.rrs, ignoreAllButTypeAndStatus); diff != "" {
				t.Error("unexpected conditions (-want, +got) =", diff)
			}
		})
	}
}

func TestRequestReplyReadyCondition(t *testing.T) {
	tests := []struct {
		name                            string
		rrs                             *RequestReplyStatus
		markAddresableSucceeded         *bool
		markTriggersReadySucceeded      *bool
		markEventPoliciesReadySucceeded *bool
		markBrokerReady                 *bool
		wantReady                       bool
	}{
		{
			name: "Initially everything is Unknown, Auth&SubjectsResolved marked as true, RR should become Ready",
			rrs: &RequestReplyStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						{Type: RequestReplyConditionReady, Status: corev1.ConditionUnknown},
						{Type: RequestReplyConditionAddressable, Status: corev1.ConditionUnknown},
						{Type: RequestReplyConditionTriggers, Status: corev1.ConditionUnknown},
						{Type: RequestReplyConditionEventPoliciesReady, Status: corev1.ConditionUnknown},
						{Type: RequestreplyConditionBrokerReady, Status: corev1.ConditionUnknown},
					},
				},
			},
			markAddresableSucceeded:         ptr.To(true),
			markTriggersReadySucceeded:      ptr.To(true),
			markEventPoliciesReadySucceeded: ptr.To(true),
			markBrokerReady:                 ptr.To(true),
			wantReady:                       true,
		},
		{
			name: "Initially everything is Ready, Addressable set to false, RR should become False",
			rrs: &RequestReplyStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						{Type: RequestReplyConditionReady, Status: corev1.ConditionTrue},
						{Type: RequestReplyConditionAddressable, Status: corev1.ConditionTrue},
						{Type: RequestReplyConditionTriggers, Status: corev1.ConditionTrue},
						{Type: RequestReplyConditionEventPoliciesReady, Status: corev1.ConditionTrue},
						{Type: RequestreplyConditionBrokerReady, Status: corev1.ConditionTrue},
					},
				},
			},
			markAddresableSucceeded:         ptr.To(false),
			markTriggersReadySucceeded:      ptr.To(true),
			markEventPoliciesReadySucceeded: ptr.To(true),
			markBrokerReady:                 ptr.To(true),
			wantReady:                       false,
		},
		{
			name: "Initially everything is Ready, Trigger set to false, RR should become False",
			rrs: &RequestReplyStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						{Type: RequestReplyConditionReady, Status: corev1.ConditionTrue},
						{Type: RequestReplyConditionAddressable, Status: corev1.ConditionTrue},
						{Type: RequestReplyConditionTriggers, Status: corev1.ConditionTrue},
						{Type: RequestReplyConditionEventPoliciesReady, Status: corev1.ConditionTrue},
						{Type: RequestreplyConditionBrokerReady, Status: corev1.ConditionTrue},
					},
				},
			},
			markAddresableSucceeded:         ptr.To(true),
			markTriggersReadySucceeded:      ptr.To(false),
			markEventPoliciesReadySucceeded: ptr.To(true),
			markBrokerReady:                 ptr.To(true),
			wantReady:                       false,
		},
		{
			name: "Initially everything is Ready, EPs set to false, RR should become False",
			rrs: &RequestReplyStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						{Type: RequestReplyConditionReady, Status: corev1.ConditionTrue},
						{Type: RequestReplyConditionAddressable, Status: corev1.ConditionTrue},
						{Type: RequestReplyConditionTriggers, Status: corev1.ConditionTrue},
						{Type: RequestReplyConditionEventPoliciesReady, Status: corev1.ConditionTrue},
						{Type: RequestreplyConditionBrokerReady, Status: corev1.ConditionTrue},
					},
				},
			},
			markAddresableSucceeded:         ptr.To(true),
			markTriggersReadySucceeded:      ptr.To(true),
			markEventPoliciesReadySucceeded: ptr.To(false),
			wantReady:                       false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.markAddresableSucceeded != nil {
				if *test.markAddresableSucceeded {
					url, _ := apis.ParseURL("http://localhost:8080")
					test.rrs.SetAddress(&duckv1.Addressable{URL: url})
				} else {
					test.rrs.SetAddress(nil)

				}
			}
			if test.markTriggersReadySucceeded != nil {
				if *test.markTriggersReadySucceeded {
					test.rrs.MarkTriggersReady()
				} else {
					test.rrs.MarkTriggersNotReadyWithReason("", "")
				}
			}
			if test.markEventPoliciesReadySucceeded != nil {
				if *test.markEventPoliciesReadySucceeded {
					test.rrs.MarkEventPoliciesTrue()
				} else {
					test.rrs.MarkEventPoliciesFailed("", "")
				}
			}
			if test.markBrokerReady != nil {
				if *test.markBrokerReady {
					test.rrs.MarkBrokerReady()
				} else {
					test.rrs.MarkBrokerNotReady("", "")
				}
			}
			rr := RequestReply{Status: *test.rrs}
			got := rr.GetConditionSet().Manage(test.rrs).IsHappy()
			if test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}
		})
	}
}
