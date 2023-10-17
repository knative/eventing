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

package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
)

var sequenceConditionReady = apis.Condition{
	Type:   SequenceConditionReady,
	Status: corev1.ConditionTrue,
}

func TestSequenceGetConditionSet(t *testing.T) {
	r := &Sequence{}

	if got, want := r.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func getSubscription(name string, ready bool) *messagingv1.Subscription {
	s := messagingv1.Subscription{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "messaging.knative.dev/v1",
			Kind:       "Subscription",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "testns",
		},
		Status: messagingv1.SubscriptionStatus{},
	}
	if ready {
		s.Status.MarkChannelReady()
		s.Status.MarkReferencesResolved()
		s.Status.MarkAddedToChannel()
		s.Status.MarkOIDCIdentityCreatedSucceeded()
	} else {
		s.Status.MarkChannelFailed("testInducedFailure", "Test Induced failure")
		s.Status.MarkReferencesNotResolved("testInducedFailure", "Test Induced failure")
		s.Status.MarkNotAddedToChannel("testInducedfailure", "Test Induced failure")
	}
	return &s
}

func getChannelable(ready bool) *eventingduckv1.Channelable {
	URL := apis.HTTP("example.com")
	c := eventingduckv1.Channelable{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "messaging.knative.dev/v1",
			Kind:       "InMemoryChannel",
		},
		ObjectMeta: metav1.ObjectMeta{},
		Status:     eventingduckv1.ChannelableStatus{},
	}

	if ready {
		c.Status.SetConditions([]apis.Condition{{
			Type:   apis.ConditionReady,
			Status: corev1.ConditionTrue,
		}})
		c.Status.Address = &duckv1.Addressable{URL: URL}
	} else {
		c.Status.SetConditions([]apis.Condition{{
			Type:   apis.ConditionReady,
			Status: corev1.ConditionUnknown,
		}})
	}

	return &c
}

func TestSequenceGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		ss        *SequenceStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name: "single condition",
		ss: &SequenceStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					sequenceConditionReady,
				},
			},
		},
		condQuery: apis.ConditionReady,
		want:      &sequenceConditionReady,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ss.GetCondition(test.condQuery)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Error("unexpected condition (-want, +got) =", diff)
			}
		})
	}
}

func TestSequenceInitializeConditions(t *testing.T) {
	tests := []struct {
		name string
		ts   *SequenceStatus
		want *SequenceStatus
	}{{
		name: "empty",
		ts:   &SequenceStatus{},
		want: &SequenceStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   SequenceConditionAddressable,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   SequenceConditionChannelsReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   SequenceConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   SequenceConditionSubscriptionsReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one false",
		ts: &SequenceStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   SequenceConditionChannelsReady,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &SequenceStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   SequenceConditionAddressable,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   SequenceConditionChannelsReady,
					Status: corev1.ConditionFalse,
				}, {
					Type:   SequenceConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   SequenceConditionSubscriptionsReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one true",
		ts: &SequenceStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   SequenceConditionSubscriptionsReady,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		want: &SequenceStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   SequenceConditionAddressable,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   SequenceConditionChannelsReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   SequenceConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   SequenceConditionSubscriptionsReady,
					Status: corev1.ConditionTrue,
				}},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.ts.InitializeConditions()
			if diff := cmp.Diff(test.want, test.ts, ignoreAllButTypeAndStatus); diff != "" {
				t.Error("unexpected conditions (-want, +got) =", diff)
			}
		})
	}
}

func TestSequencePropagateSubscriptionStatuses(t *testing.T) {
	tests := []struct {
		name string
		subs []*messagingv1.Subscription
		want corev1.ConditionStatus
	}{{
		name: "empty",
		subs: []*messagingv1.Subscription{},
		want: corev1.ConditionUnknown,
	}, {
		name: "empty status",
		subs: []*messagingv1.Subscription{{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "messaging.knative.dev/v1",
				Kind:       "Subscription",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sub",
				Namespace: "testns",
			},
			Status: messagingv1.SubscriptionStatus{},
		},
		},
		want: corev1.ConditionUnknown,
	}, {
		name: "one subscription not ready",
		subs: []*messagingv1.Subscription{getSubscription("sub0", false)},
		want: corev1.ConditionUnknown,
	}, {
		name: "one subscription ready",
		subs: []*messagingv1.Subscription{getSubscription("sub0", true)},
		want: corev1.ConditionTrue,
	}, {
		name: "one subscription ready, one not",
		subs: []*messagingv1.Subscription{getSubscription("sub0", true), getSubscription("sub1", false)},
		want: corev1.ConditionUnknown,
	}, {
		name: "two subscriptions ready",
		subs: []*messagingv1.Subscription{getSubscription("sub0", true), getSubscription("sub1", true)},
		want: corev1.ConditionTrue,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ps := SequenceStatus{}
			ps.PropagateSubscriptionStatuses(test.subs)
			got := ps.GetCondition(SequenceConditionSubscriptionsReady).Status
			want := test.want
			if want != got {
				t.Errorf("unexpected conditions (-want, +got) = %v %v", want, got)
			}
		})
	}
}

func TestSequencePropagateChannelStatuses(t *testing.T) {
	tests := []struct {
		name     string
		channels []*eventingduckv1.Channelable
		want     corev1.ConditionStatus
	}{{
		name:     "empty",
		channels: []*eventingduckv1.Channelable{},
		want:     corev1.ConditionUnknown,
	}, {
		name:     "one channelable not ready",
		channels: []*eventingduckv1.Channelable{getChannelable(false)},
		want:     corev1.ConditionUnknown,
	}, {
		name:     "one channelable ready",
		channels: []*eventingduckv1.Channelable{getChannelable(true)},
		want:     corev1.ConditionTrue,
	}, {
		name:     "one channelable ready, one not",
		channels: []*eventingduckv1.Channelable{getChannelable(true), getChannelable(false)},
		want:     corev1.ConditionUnknown,
	}, {
		name:     "two channelables ready",
		channels: []*eventingduckv1.Channelable{getChannelable(true), getChannelable(true)},
		want:     corev1.ConditionTrue,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ps := SequenceStatus{}
			ps.PropagateChannelStatuses(test.channels)
			got := ps.GetCondition(SequenceConditionChannelsReady).Status
			want := test.want
			if want != got {
				t.Errorf("unexpected conditions: want=%q, got=%q", want, got)
			}
		})
	}
}

func TestSequenceReady(t *testing.T) {
	tests := []struct {
		name     string
		subs     []*messagingv1.Subscription
		channels []*eventingduckv1.Channelable
		want     bool
	}{{
		name:     "empty",
		subs:     []*messagingv1.Subscription{},
		channels: []*eventingduckv1.Channelable{},
		want:     false,
	}, {
		name:     "one channelable not ready, one subscription ready",
		channels: []*eventingduckv1.Channelable{getChannelable(false)},
		subs:     []*messagingv1.Subscription{getSubscription("sub0", true)},
		want:     false,
	}, {
		name:     "one channelable ready, one subscription not ready",
		channels: []*eventingduckv1.Channelable{getChannelable(true)},
		subs:     []*messagingv1.Subscription{getSubscription("sub0", false)},
		want:     false,
	}, {
		name:     "one channelable ready, one subscription ready",
		channels: []*eventingduckv1.Channelable{getChannelable(true)},
		subs:     []*messagingv1.Subscription{getSubscription("sub0", true)},
		want:     true,
	}, {
		name:     "one channelable ready, one not, two subscriptions ready",
		channels: []*eventingduckv1.Channelable{getChannelable(true), getChannelable(false)},
		subs:     []*messagingv1.Subscription{getSubscription("sub0", true), getSubscription("sub1", true)},
		want:     false,
	}, {
		name:     "two channelables ready, one subscription ready, one not",
		channels: []*eventingduckv1.Channelable{getChannelable(true), getChannelable(true)},
		subs:     []*messagingv1.Subscription{getSubscription("sub0", true), getSubscription("sub1", false)},
		want:     false,
	}, {
		name:     "two channelables ready, two subscriptions ready",
		channels: []*eventingduckv1.Channelable{getChannelable(true), getChannelable(true)},
		subs:     []*messagingv1.Subscription{getSubscription("sub0", true), getSubscription("sub1", true)},
		want:     true,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ps := SequenceStatus{}
			ps.PropagateChannelStatuses(test.channels)
			ps.PropagateSubscriptionStatuses(test.subs)
			got := ps.IsReady()
			want := test.want
			if want != got {
				t.Errorf("unexpected conditions (-want, +got) = %v %v", want, got)
			}
		})
	}
}

func TestSequencePropagateSetAddress(t *testing.T) {
	URL := apis.HTTP("example.com")
	URL2 := apis.HTTP("another.example.com")
	tests := []struct {
		name        string
		status      SequenceStatus
		address     *duckv1.Addressable
		want        duckv1.Addressable
		wantStatus  corev1.ConditionStatus
		wantAddress string
	}{{
		name:       "nil",
		status:     SequenceStatus{},
		address:    nil,
		want:       duckv1.Addressable{},
		wantStatus: corev1.ConditionUnknown,
	}, {
		name:       "empty",
		status:     SequenceStatus{},
		address:    &duckv1.Addressable{},
		want:       duckv1.Addressable{},
		wantStatus: corev1.ConditionUnknown,
	}, {
		name:        "URL",
		status:      SequenceStatus{},
		address:     &duckv1.Addressable{URL: URL},
		want:        duckv1.Addressable{URL: URL},
		wantStatus:  corev1.ConditionTrue,
		wantAddress: "http://example.com",
	}, {
		name: "New URL",
		status: SequenceStatus{
			Address: duckv1.Addressable{
				URL: URL2,
			},
		},
		address:     &duckv1.Addressable{URL: URL},
		want:        duckv1.Addressable{URL: URL},
		wantStatus:  corev1.ConditionTrue,
		wantAddress: "http://example.com",
	}, {
		name: "Clear URL",
		status: SequenceStatus{
			Address: duckv1.Addressable{
				URL: URL,
			},
		},
		address:    nil,
		want:       duckv1.Addressable{},
		wantStatus: corev1.ConditionUnknown,
	}, {
		name:       "nil",
		status:     SequenceStatus{},
		address:    &duckv1.Addressable{URL: nil},
		want:       duckv1.Addressable{},
		wantStatus: corev1.ConditionUnknown,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			tt.status.setAddress(tt.address)
			got := tt.status.Address
			if diff := cmp.Diff(tt.want, got, ignoreAllButTypeAndStatus); diff != "" {
				t.Error("unexpected address (-want, +got) =", diff)
			}
			gotStatus := tt.status.GetCondition(SequenceConditionAddressable).Status
			if tt.wantStatus != gotStatus {
				t.Errorf("unexpected conditions (-want, +got) = %v %v", tt.wantStatus, gotStatus)
			}
			gotAddress := ""
			if got.URL != nil {
				gotAddress = got.URL.String()
			}
			if diff := cmp.Diff(tt.wantAddress, gotAddress); diff != "" {
				t.Error("unexpected address.url (-want, +got) =", diff)
			}
		})
	}
}
