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

	"knative.dev/pkg/apis"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgduckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

var parallelConditionReady = apis.Condition{
	Type:   ParallelConditionReady,
	Status: corev1.ConditionTrue,
}

var parallelConditionChannelsReady = apis.Condition{
	Type:   ParallelConditionChannelsReady,
	Status: corev1.ConditionTrue,
}

var parallelConditionSubscriptionsReady = apis.Condition{
	Type:   ParallelConditionSubscriptionsReady,
	Status: corev1.ConditionTrue,
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
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
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
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}

func TestParallelPropagateSubscriptionStatuses(t *testing.T) {
	tests := []struct {
		name  string
		fsubs []*Subscription
		subs  []*Subscription
		want  corev1.ConditionStatus
	}{{
		name:  "empty",
		fsubs: []*Subscription{},
		subs:  []*Subscription{},
		want:  corev1.ConditionFalse,
	}, {
		name: "empty status",
		fsubs: []*Subscription{{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "messaging.knative.dev/v1alpha1",
				Kind:       "Subscription",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sub",
				Namespace: "testns",
			},
			Status: SubscriptionStatus{},
		}}, subs: []*Subscription{{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "messaging.knative.dev/v1alpha1",
				Kind:       "Subscription",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sub",
				Namespace: "testns",
			},
			Status: SubscriptionStatus{},
		}},
		want: corev1.ConditionFalse,
	}, {
		name:  "one filter and subscriber subscription not ready",
		fsubs: []*Subscription{getSubscription("fsub0", false)},
		subs:  []*Subscription{getSubscription("sub0", false)},
		want:  corev1.ConditionFalse,
	}, {
		name:  "one filter and one subscription ready",
		fsubs: []*Subscription{getSubscription("fsub0", true)},
		subs:  []*Subscription{getSubscription("sub0", true)},
		want:  corev1.ConditionTrue,
	}, {
		name:  "one filter subscription not ready and one subscription ready",
		fsubs: []*Subscription{getSubscription("fsub0", false)},
		subs:  []*Subscription{getSubscription("sub0", true)},
		want:  corev1.ConditionFalse,
	}, {
		name:  "one subscription ready, one not",
		fsubs: []*Subscription{getSubscription("fsub0", true), getSubscription("fsub1", false)},
		subs:  []*Subscription{getSubscription("sub0", true), getSubscription("sub1", false)},
		want:  corev1.ConditionFalse,
	}, {
		name:  "two subscriptions ready",
		fsubs: []*Subscription{getSubscription("fsub0", true), getSubscription("fsub1", true)},
		subs:  []*Subscription{getSubscription("sub0", true), getSubscription("sub1", true)},
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

func TestParallelPropagateChannelStatuses(t *testing.T) {
	tests := []struct {
		name     string
		ichannel *duckv1alpha1.Channelable
		channels []*duckv1alpha1.Channelable
		want     corev1.ConditionStatus
	}{{
		name:     "ingress false, empty",
		ichannel: getChannelable(false),
		channels: []*duckv1alpha1.Channelable{},
		want:     corev1.ConditionFalse,
	}, {
		name:     "ingress false, one channelable not ready",
		ichannel: getChannelable(false),
		channels: []*duckv1alpha1.Channelable{getChannelable(false)},
		want:     corev1.ConditionFalse,
	}, {
		name:     "ingress true, one channelable not ready",
		ichannel: getChannelable(true),
		channels: []*duckv1alpha1.Channelable{getChannelable(false)},
		want:     corev1.ConditionFalse,
	}, {
		name:     "ingress false, one channelable ready",
		ichannel: getChannelable(false),
		channels: []*duckv1alpha1.Channelable{getChannelable(true)},
		want:     corev1.ConditionFalse,
	}, {
		name:     "ingress true, one channelable ready",
		ichannel: getChannelable(true),
		channels: []*duckv1alpha1.Channelable{getChannelable(true)},
		want:     corev1.ConditionTrue,
	}, {
		name:     "ingress true, one channelable ready, one not",
		ichannel: getChannelable(true),
		channels: []*duckv1alpha1.Channelable{getChannelable(true), getChannelable(false)},
		want:     corev1.ConditionFalse,
	}, {
		name:     "ingress true, two channelables ready",
		ichannel: getChannelable(true),
		channels: []*duckv1alpha1.Channelable{getChannelable(true), getChannelable(true)},
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

func TestParallelReady(t *testing.T) {
	tests := []struct {
		name     string
		fsubs    []*Subscription
		subs     []*Subscription
		ichannel *duckv1alpha1.Channelable
		channels []*duckv1alpha1.Channelable
		want     bool
	}{{
		name:     "ingress false, empty",
		fsubs:    []*Subscription{},
		subs:     []*Subscription{},
		ichannel: getChannelable(false),
		channels: []*duckv1alpha1.Channelable{},
		want:     false,
	}, {
		name:     "ingress true, empty",
		fsubs:    []*Subscription{},
		subs:     []*Subscription{},
		ichannel: getChannelable(true),
		channels: []*duckv1alpha1.Channelable{},
		want:     false,
	}, {
		name:     "ingress true, one channelable not ready, one subscription ready",
		ichannel: getChannelable(true),
		channels: []*duckv1alpha1.Channelable{getChannelable(false)},
		fsubs:    []*Subscription{getSubscription("fsub0", true)},
		subs:     []*Subscription{getSubscription("sub0", true)},
		want:     false,
	}, {
		name:     "ingress true, one channelable ready, one subscription not ready",
		ichannel: getChannelable(true),
		channels: []*duckv1alpha1.Channelable{getChannelable(true)},
		fsubs:    []*Subscription{getSubscription("fsub0", false)},
		subs:     []*Subscription{getSubscription("sub0", false)},
		want:     false,
	}, {
		name:     "ingress false, one channelable ready, one subscription ready",
		ichannel: getChannelable(false),
		channels: []*duckv1alpha1.Channelable{getChannelable(true)},
		fsubs:    []*Subscription{getSubscription("fsub0", true)},
		subs:     []*Subscription{getSubscription("sub0", true)},
		want:     false,
	}, {
		name:     "ingress true, one channelable ready, one subscription ready",
		ichannel: getChannelable(true),
		channels: []*duckv1alpha1.Channelable{getChannelable(true)},
		fsubs:    []*Subscription{getSubscription("fsub0", true)},
		subs:     []*Subscription{getSubscription("sub0", true)},
		want:     true,
	}, {
		name:     "ingress true, one channelable ready, one not, two subsriptions ready",
		ichannel: getChannelable(true),
		channels: []*duckv1alpha1.Channelable{getChannelable(true), getChannelable(false)},
		fsubs:    []*Subscription{getSubscription("fsub0", true), getSubscription("fsub1", true)},
		subs:     []*Subscription{getSubscription("sub0", true), getSubscription("sub1", true)},
		want:     false,
	}, {
		name:     "ingress true, two channelables ready, one subscription ready, one not",
		ichannel: getChannelable(true),
		channels: []*duckv1alpha1.Channelable{getChannelable(true), getChannelable(true)},
		fsubs:    []*Subscription{getSubscription("fsub0", true), getSubscription("fsub1", false)},
		subs:     []*Subscription{getSubscription("sub0", true), getSubscription("sub1", false)},
		want:     false,
	}, {
		name:     "ingress false, two channelables ready, two subscriptions ready",
		ichannel: getChannelable(false),
		channels: []*duckv1alpha1.Channelable{getChannelable(true), getChannelable(true)},
		fsubs:    []*Subscription{getSubscription("fsub0", true), getSubscription("fsub1", true)},
		subs:     []*Subscription{getSubscription("sub0", true), getSubscription("sub1", true)},
		want:     false,
	}, {
		name:     "ingress true, two channelables ready, two subscriptions ready",
		ichannel: getChannelable(true),
		channels: []*duckv1alpha1.Channelable{getChannelable(true), getChannelable(true)},
		fsubs:    []*Subscription{getSubscription("fsub0", true), getSubscription("fsub1", true)},
		subs:     []*Subscription{getSubscription("sub0", true), getSubscription("sub1", true)},
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
	URL, _ := apis.ParseURL("http://example.com")
	tests := []struct {
		name       string
		address    *pkgduckv1alpha1.Addressable
		want       *pkgduckv1alpha1.Addressable
		wantStatus corev1.ConditionStatus
	}{{
		name:       "nil",
		address:    nil,
		want:       nil,
		wantStatus: corev1.ConditionFalse,
	}, {
		name:       "empty",
		address:    &pkgduckv1alpha1.Addressable{},
		want:       &pkgduckv1alpha1.Addressable{},
		wantStatus: corev1.ConditionFalse,
	}, {
		name:       "URL",
		address:    &pkgduckv1alpha1.Addressable{duckv1beta1.Addressable{URL}, ""},
		want:       &pkgduckv1alpha1.Addressable{duckv1beta1.Addressable{URL}, ""},
		wantStatus: corev1.ConditionTrue,
	}, {
		name:       "hostname",
		address:    &pkgduckv1alpha1.Addressable{duckv1beta1.Addressable{}, "myhostname"},
		want:       &pkgduckv1alpha1.Addressable{duckv1beta1.Addressable{}, "myhostname"},
		wantStatus: corev1.ConditionTrue,
	}, {
		name:       "nil",
		address:    &pkgduckv1alpha1.Addressable{duckv1beta1.Addressable{nil}, ""},
		want:       &pkgduckv1alpha1.Addressable{duckv1beta1.Addressable{}, ""},
		wantStatus: corev1.ConditionFalse,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ps := ParallelStatus{}
			ps.setAddress(test.address)
			got := ps.Address
			if diff := cmp.Diff(test.want, got, ignoreAllButTypeAndStatus); diff != "" {
				t.Errorf("unexpected address (-want, +got) = %v", diff)
			}
			gotStatus := ps.GetCondition(ParallelConditionAddressable).Status
			if test.wantStatus != gotStatus {
				t.Errorf("unexpected conditions (-want, +got) = %v %v", test.wantStatus, gotStatus)
			}
		})
	}
}
