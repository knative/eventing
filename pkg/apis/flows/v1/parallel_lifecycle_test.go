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

	"knative.dev/pkg/ptr"

	"knative.dev/pkg/apis"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var parallelConditionReady = apis.Condition{
	Type:   ParallelConditionReady,
	Status: corev1.ConditionTrue,
}

func TestParallelGetConditionSet(t *testing.T) {
	r := &Parallel{}

	if got, want := r.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func TestParallelGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		ss        *ParallelStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name: "single condition",
		ss: &ParallelStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					parallelConditionReady,
				},
			},
		},
		condQuery: apis.ConditionReady,
		want:      &parallelConditionReady,
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

func TestParallelInitializeConditions(t *testing.T) {
	tests := []struct {
		name string
		ts   *ParallelStatus
		want *ParallelStatus
	}{{
		name: "empty",
		ts:   &ParallelStatus{},
		want: &ParallelStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   ParallelConditionAddressable,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   ParallelConditionChannelsReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   ParallelConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   ParallelConditionSubscriptionsReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one false",
		ts: &ParallelStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   ParallelConditionChannelsReady,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &ParallelStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   ParallelConditionAddressable,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   ParallelConditionChannelsReady,
					Status: corev1.ConditionFalse,
				}, {
					Type:   ParallelConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   ParallelConditionSubscriptionsReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one true",
		ts: &ParallelStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   ParallelConditionSubscriptionsReady,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		want: &ParallelStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   ParallelConditionAddressable,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   ParallelConditionChannelsReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   ParallelConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   ParallelConditionSubscriptionsReady,
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

func TestParallelPropagateSubscriptionStatuses(t *testing.T) {
	tests := []struct {
		name  string
		fsubs []*messagingv1.Subscription
		subs  []*messagingv1.Subscription
		want  corev1.ConditionStatus
	}{{
		name:  "empty",
		fsubs: []*messagingv1.Subscription{},
		subs:  []*messagingv1.Subscription{},
		want:  corev1.ConditionFalse,
	}, {
		name: "empty status",
		fsubs: []*messagingv1.Subscription{{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "messaging.knative.dev/v1",
				Kind:       "Subscription",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sub",
				Namespace: "testns",
			},
			Status: messagingv1.SubscriptionStatus{},
		}}, subs: []*messagingv1.Subscription{{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "messaging.knative.dev/v1",
				Kind:       "Subscription",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sub",
				Namespace: "testns",
			},
			Status: messagingv1.SubscriptionStatus{},
		}},
		want: corev1.ConditionFalse,
	}, {
		name:  "one filter and subscriber subscription not ready",
		fsubs: []*messagingv1.Subscription{getSubscription("fsub0", false)},
		subs:  []*messagingv1.Subscription{getSubscription("sub0", false)},
		want:  corev1.ConditionFalse,
	}, {
		name:  "one filter and one subscription ready",
		fsubs: []*messagingv1.Subscription{getSubscription("fsub0", true)},
		subs:  []*messagingv1.Subscription{getSubscription("sub0", true)},
		want:  corev1.ConditionTrue,
	}, {
		name:  "one filter subscription not ready and one subscription ready",
		fsubs: []*messagingv1.Subscription{getSubscription("fsub0", false)},
		subs:  []*messagingv1.Subscription{getSubscription("sub0", true)},
		want:  corev1.ConditionFalse,
	}, {
		name:  "one subscription ready, one not",
		fsubs: []*messagingv1.Subscription{getSubscription("fsub0", true), getSubscription("fsub1", false)},
		subs:  []*messagingv1.Subscription{getSubscription("sub0", true), getSubscription("sub1", false)},
		want:  corev1.ConditionFalse,
	}, {
		name:  "two subscriptions ready",
		fsubs: []*messagingv1.Subscription{getSubscription("fsub0", true), getSubscription("fsub1", true)},
		subs:  []*messagingv1.Subscription{getSubscription("sub0", true), getSubscription("sub1", true)},
		want:  corev1.ConditionTrue,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ps := ParallelStatus{}
			ps.PropagateSubscriptionStatuses(test.fsubs, test.subs)
			got := ps.GetCondition(ParallelConditionSubscriptionsReady).Status
			want := test.want
			if want != got {
				t.Errorf("unexpected conditions (-want, +got) = %v %v", want, got)
			}
		})
	}
}

func TestParallelPropagateSubscriptionOIDCServiceAccounts(t *testing.T) {
	tests := []struct {
		name        string
		filterSubs  []*messagingv1.Subscription
		subs        []*messagingv1.Subscription
		wantOIDCSAs []string
	}{{
		name:       "empty",
		filterSubs: []*messagingv1.Subscription{},
		subs:       []*messagingv1.Subscription{},
	}, {
		name: "both subscriptions with OIDC SAs",
		filterSubs: []*messagingv1.Subscription{{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "messaging.knative.dev/v1",
				Kind:       "Subscription",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sub",
				Namespace: "testns",
			},
			Status: messagingv1.SubscriptionStatus{
				Auth: &duckv1.AuthStatus{
					ServiceAccountName: ptr.String("filterSub-oidc-sa"),
				},
			},
		}}, subs: []*messagingv1.Subscription{{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "messaging.knative.dev/v1",
				Kind:       "Subscription",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sub",
				Namespace: "testns",
			},
			Status: messagingv1.SubscriptionStatus{
				Auth: &duckv1.AuthStatus{
					ServiceAccountName: ptr.String("sub-oidc-sa"),
				},
			},
		}},
		wantOIDCSAs: []string{
			"filterSub-oidc-sa",
			"sub-oidc-sa",
		},
	}, {
		name:       "filter subscription without OIDC SA",
		filterSubs: []*messagingv1.Subscription{getSubscription("fsub0", false)},
		subs: []*messagingv1.Subscription{{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "messaging.knative.dev/v1",
				Kind:       "Subscription",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sub",
				Namespace: "testns",
			},
			Status: messagingv1.SubscriptionStatus{
				Auth: &duckv1.AuthStatus{
					ServiceAccountName: ptr.String("sub-oidc-sa"),
				},
			},
		}},
		wantOIDCSAs: []string{
			"sub-oidc-sa",
		},
	}, {
		name: "subscriber subscription without OIDC SA",
		filterSubs: []*messagingv1.Subscription{{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "messaging.knative.dev/v1",
				Kind:       "Subscription",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sub",
				Namespace: "testns",
			},
			Status: messagingv1.SubscriptionStatus{
				Auth: &duckv1.AuthStatus{
					ServiceAccountName: ptr.String("filterSub-oidc-sa"),
				},
			},
		}},
		subs: []*messagingv1.Subscription{getSubscription("sub0", false)},
		wantOIDCSAs: []string{
			"filterSub-oidc-sa",
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ps := ParallelStatus{}
			ps.PropagateSubscriptionStatuses(test.filterSubs, test.subs)

			var got []string
			if ps.Auth != nil {
				got = ps.Auth.ServiceAccountNames
			}

			if diff := cmp.Diff(test.wantOIDCSAs, got); diff != "" {
				t.Errorf("unexpected OIDC service accounts (-want, +got) = %v", diff)
			}
		})
	}
}

func TestParallelPropagateChannelStatuses(t *testing.T) {
	tests := []struct {
		name     string
		ichannel *eventingduckv1.Channelable
		channels []*eventingduckv1.Channelable
		want     corev1.ConditionStatus
	}{{
		name:     "ingress false, empty",
		ichannel: getChannelable(false),
		channels: []*eventingduckv1.Channelable{},
		want:     corev1.ConditionFalse,
	}, {
		name:     "ingress false, one channelable not ready",
		ichannel: getChannelable(false),
		channels: []*eventingduckv1.Channelable{getChannelable(false)},
		want:     corev1.ConditionFalse,
	}, {
		name:     "ingress true, one channelable not ready",
		ichannel: getChannelable(true),
		channels: []*eventingduckv1.Channelable{getChannelable(false)},
		want:     corev1.ConditionFalse,
	}, {
		name:     "ingress false, one channelable ready",
		ichannel: getChannelable(false),
		channels: []*eventingduckv1.Channelable{getChannelable(true)},
		want:     corev1.ConditionFalse,
	}, {
		name:     "ingress true, one channelable ready",
		ichannel: getChannelable(true),
		channels: []*eventingduckv1.Channelable{getChannelable(true)},
		want:     corev1.ConditionTrue,
	}, {
		name:     "ingress true, one channelable ready, one not",
		ichannel: getChannelable(true),
		channels: []*eventingduckv1.Channelable{getChannelable(true), getChannelable(false)},
		want:     corev1.ConditionFalse,
	}, {
		name:     "ingress true, two channelables ready",
		ichannel: getChannelable(true),
		channels: []*eventingduckv1.Channelable{getChannelable(true), getChannelable(true)},
		want:     corev1.ConditionTrue,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ps := ParallelStatus{}
			ps.PropagateChannelStatuses(test.ichannel, test.channels)
			got := ps.GetCondition(ParallelConditionChannelsReady).Status
			want := test.want
			if want != got {
				t.Errorf("unexpected conditions (-want, +got) = %v %v", want, got)
			}
		})
	}
}

func TestParallelPropagateChannelStatusUpdated(t *testing.T) {
	inChannel := getChannelable(true)
	initialChannels := []*eventingduckv1.Channelable{getChannelable(true)}
	afterChannels := []*eventingduckv1.Channelable{getChannelable(true), getChannelable(true)}
	ps := ParallelStatus{}
	ps.PropagateChannelStatuses(inChannel, initialChannels)
	if len(ps.BranchStatuses) != 1 {
		t.Error("unexpected branchstatuses want 1 got", len(ps.BranchStatuses))
	}
	ps.PropagateChannelStatuses(inChannel, afterChannels)
	if len(ps.BranchStatuses) != 2 {
		t.Error("unexpected branchstatuses want 2 got", len(ps.BranchStatuses))
	}
}

func TestParallelPropagateSubscriptionStatusUpdated(t *testing.T) {
	initialFsubs := []*messagingv1.Subscription{getSubscription("fsub0", true)}
	initialSubs := []*messagingv1.Subscription{getSubscription("sub0", true)}
	afterFsubs := []*messagingv1.Subscription{getSubscription("fsub0", true), getSubscription("fsub1", true)}
	afterSubs := []*messagingv1.Subscription{getSubscription("sub0", true), getSubscription("sub1", true)}
	ps := ParallelStatus{}
	ps.PropagateSubscriptionStatuses(initialFsubs, initialSubs)
	if len(ps.BranchStatuses) != 1 {
		t.Error("unexpected branchstatuses want 1 got", len(ps.BranchStatuses))
	}
	ps.PropagateSubscriptionStatuses(afterFsubs, afterSubs)
	if len(ps.BranchStatuses) != 2 {
		t.Error("unexpected branchstatuses want 2 got", len(ps.BranchStatuses))
	}
}

func TestParallelReady(t *testing.T) {
	tests := []struct {
		name     string
		fsubs    []*messagingv1.Subscription
		subs     []*messagingv1.Subscription
		ichannel *eventingduckv1.Channelable
		channels []*eventingduckv1.Channelable
		want     bool
	}{{
		name:     "ingress false, empty",
		fsubs:    []*messagingv1.Subscription{},
		subs:     []*messagingv1.Subscription{},
		ichannel: getChannelable(false),
		channels: []*eventingduckv1.Channelable{},
		want:     false,
	}, {
		name:     "ingress true, empty",
		fsubs:    []*messagingv1.Subscription{},
		subs:     []*messagingv1.Subscription{},
		ichannel: getChannelable(true),
		channels: []*eventingduckv1.Channelable{},
		want:     false,
	}, {
		name:     "ingress true, one channelable not ready, one subscription ready",
		ichannel: getChannelable(true),
		channels: []*eventingduckv1.Channelable{getChannelable(false)},
		fsubs:    []*messagingv1.Subscription{getSubscription("fsub0", true)},
		subs:     []*messagingv1.Subscription{getSubscription("sub0", true)},
		want:     false,
	}, {
		name:     "ingress true, one channelable ready, one subscription not ready",
		ichannel: getChannelable(true),
		channels: []*eventingduckv1.Channelable{getChannelable(true)},
		fsubs:    []*messagingv1.Subscription{getSubscription("fsub0", false)},
		subs:     []*messagingv1.Subscription{getSubscription("sub0", false)},
		want:     false,
	}, {
		name:     "ingress false, one channelable ready, one subscription ready",
		ichannel: getChannelable(false),
		channels: []*eventingduckv1.Channelable{getChannelable(true)},
		fsubs:    []*messagingv1.Subscription{getSubscription("fsub0", true)},
		subs:     []*messagingv1.Subscription{getSubscription("sub0", true)},
		want:     false,
	}, {
		name:     "ingress true, one channelable ready, one subscription ready",
		ichannel: getChannelable(true),
		channels: []*eventingduckv1.Channelable{getChannelable(true)},
		fsubs:    []*messagingv1.Subscription{getSubscription("fsub0", true)},
		subs:     []*messagingv1.Subscription{getSubscription("sub0", true)},
		want:     true,
	}, {
		name:     "ingress true, one channelable ready, one not, two subscriptions ready",
		ichannel: getChannelable(true),
		channels: []*eventingduckv1.Channelable{getChannelable(true), getChannelable(false)},
		fsubs:    []*messagingv1.Subscription{getSubscription("fsub0", true), getSubscription("fsub1", true)},
		subs:     []*messagingv1.Subscription{getSubscription("sub0", true), getSubscription("sub1", true)},
		want:     false,
	}, {
		name:     "ingress true, two channelables ready, one subscription ready, one not",
		ichannel: getChannelable(true),
		channels: []*eventingduckv1.Channelable{getChannelable(true), getChannelable(true)},
		fsubs:    []*messagingv1.Subscription{getSubscription("fsub0", true), getSubscription("fsub1", false)},
		subs:     []*messagingv1.Subscription{getSubscription("sub0", true), getSubscription("sub1", false)},
		want:     false,
	}, {
		name:     "ingress false, two channelables ready, two subscriptions ready",
		ichannel: getChannelable(false),
		channels: []*eventingduckv1.Channelable{getChannelable(true), getChannelable(true)},
		fsubs:    []*messagingv1.Subscription{getSubscription("fsub0", true), getSubscription("fsub1", true)},
		subs:     []*messagingv1.Subscription{getSubscription("sub0", true), getSubscription("sub1", true)},
		want:     false,
	}, {
		name:     "ingress true, two channelables ready, two subscriptions ready",
		ichannel: getChannelable(true),
		channels: []*eventingduckv1.Channelable{getChannelable(true), getChannelable(true)},
		fsubs:    []*messagingv1.Subscription{getSubscription("fsub0", true), getSubscription("fsub1", true)},
		subs:     []*messagingv1.Subscription{getSubscription("sub0", true), getSubscription("sub1", true)},
		want:     true,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ps := ParallelStatus{}
			ps.PropagateChannelStatuses(test.ichannel, test.channels)
			ps.PropagateSubscriptionStatuses(test.fsubs, test.subs)
			got := ps.IsReady()
			want := test.want
			if want != got {
				t.Errorf("unexpected conditions (-want, +got) = %v %v", want, got)
			}
		})
	}
}

func TestParallelPropagateSetAddress(t *testing.T) {
	URL := apis.HTTP("example.com")
	audience := pointer.String("foo-bar")
	tests := []struct {
		name       string
		address    *duckv1.Addressable
		want       *duckv1.Addressable
		wantStatus corev1.ConditionStatus
	}{{
		name:       "nil",
		address:    nil,
		want:       nil,
		wantStatus: corev1.ConditionFalse,
	}, {
		name:       "empty",
		address:    &duckv1.Addressable{},
		want:       &duckv1.Addressable{},
		wantStatus: corev1.ConditionTrue,
	}, {
		name:       "URL",
		address:    &duckv1.Addressable{URL: URL},
		want:       &duckv1.Addressable{URL: URL},
		wantStatus: corev1.ConditionTrue,
	}, {
		name:       "nil",
		address:    &duckv1.Addressable{URL: nil},
		want:       &duckv1.Addressable{},
		wantStatus: corev1.ConditionTrue,
	}, {
		name:       "Audience",
		address:    &duckv1.Addressable{Audience: audience},
		want:       &duckv1.Addressable{Audience: audience},
		wantStatus: corev1.ConditionTrue,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ps := ParallelStatus{}
			ps.setAddress(test.address)
			got := ps.Address
			if diff := cmp.Diff(test.want, got, ignoreAllButTypeAndStatus); diff != "" {
				t.Error("unexpected address (-want, +got) =", diff)
			}
			gotStatus := ps.GetCondition(ParallelConditionAddressable).Status
			if test.wantStatus != gotStatus {
				t.Errorf("unexpected conditions (-want, +got) = %v %v", test.wantStatus, gotStatus)
			}
		})
	}
}
