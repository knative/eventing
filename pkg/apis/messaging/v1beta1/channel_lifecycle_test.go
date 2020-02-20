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

package v1beta1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	validAddress = &duckv1.Addressable{
		URL: &apis.URL{
			Scheme: "http",
			Host:   "test-domain",
		},
	}
)

func TestChannelGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		cs        *ChannelStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name: "single condition",
		cs: &ChannelStatus{
			ChannelableStatus: v1beta1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						condReady,
					},
				},
			},
		},
		condQuery: apis.ConditionReady,
		want:      &condReady,
	}, {
		name: "unknown condition",
		cs: &ChannelStatus{
			ChannelableStatus: v1beta1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						condReady,
					},
				},
			},
		},
		condQuery: apis.ConditionType("foo"),
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
			ChannelableStatus: v1beta1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   ChannelConditionAddressable,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   ChannelConditionBackingChannelReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   ChannelConditionReady,
						Status: corev1.ConditionUnknown,
					}},
				},
			},
		},
	}, {
		name: "one false",
		cs: &ChannelStatus{
			ChannelableStatus: v1beta1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   ChannelConditionAddressable,
						Status: corev1.ConditionFalse,
					}},
				},
			},
		},
		want: &ChannelStatus{
			ChannelableStatus: v1beta1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   ChannelConditionAddressable,
						Status: corev1.ConditionFalse,
					}, {
						Type:   ChannelConditionBackingChannelReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   ChannelConditionReady,
						Status: corev1.ConditionUnknown,
					}},
				},
			},
		},
	}, {
		name: "one true",
		cs: &ChannelStatus{
			ChannelableStatus: v1beta1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   ChannelConditionBackingChannelReady,
						Status: corev1.ConditionTrue,
					}},
				},
			},
		},
		want: &ChannelStatus{
			ChannelableStatus: v1beta1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   ChannelConditionAddressable,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   ChannelConditionBackingChannelReady,
						Status: corev1.ConditionTrue,
					}, {
						Type:   ChannelConditionReady,
						Status: corev1.ConditionUnknown,
					}},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.cs.InitializeConditions()
			ignore := cmpopts.IgnoreFields(
				apis.Condition{},
				"LastTransitionTime", "Message", "Reason", "Severity")
			if diff := cmp.Diff(test.want, test.cs, ignore); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}

func TestChannelConditionStatus(t *testing.T) {
	tests := []struct {
		name                 string
		address              *duckv1.Addressable
		backingChannelStatus corev1.ConditionStatus
		wantConditionStatus  corev1.ConditionStatus
	}{{
		name:                 "all happy",
		address:              validAddress,
		backingChannelStatus: corev1.ConditionTrue,
		wantConditionStatus:  corev1.ConditionTrue,
	}, {
		name:                 "address not set",
		address:              &duckv1.Addressable{},
		backingChannelStatus: corev1.ConditionTrue,
		wantConditionStatus:  corev1.ConditionFalse,
	},
		{
			name:                 "nil address",
			address:              nil,
			backingChannelStatus: corev1.ConditionTrue,
			wantConditionStatus:  corev1.ConditionFalse,
		}, {
			name:                 "backing channel with unknown status",
			address:              validAddress,
			backingChannelStatus: corev1.ConditionUnknown,
			wantConditionStatus:  corev1.ConditionUnknown,
		}, {
			name:                 "backing channel with false status",
			address:              validAddress,
			backingChannelStatus: corev1.ConditionFalse,
			wantConditionStatus:  corev1.ConditionFalse,
		}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cs := &ChannelStatus{}
			cs.InitializeConditions()
			cs.SetAddress(test.address)
			if test.backingChannelStatus == corev1.ConditionTrue {
				cs.MarkBackingChannelReady()
			} else if test.backingChannelStatus == corev1.ConditionFalse {
				cs.MarkBackingChannelFailed("ChannelFailure", "testing")
			} else {
				cs.MarkBackingChannelUnknown("ChannelUnknown", "testing")
			}
			got := cs.GetTopLevelCondition().Status
			if test.wantConditionStatus != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantConditionStatus, got)
			}
		})
	}
}

func TestChannelSetAddressable(t *testing.T) {
	testCases := map[string]struct {
		address *duckv1.Addressable
		want    *ChannelStatus
	}{
		"nil url": {
			want: &ChannelStatus{
				ChannelableStatus: v1beta1.ChannelableStatus{
					Status: duckv1.Status{
						Conditions: []apis.Condition{
							{
								Type:   ChannelConditionAddressable,
								Status: corev1.ConditionFalse,
							},
							// Note that Ready is here because when the condition is marked False, duck
							// automatically sets Ready to false.
							{
								Type:   ChannelConditionReady,
								Status: corev1.ConditionFalse,
							},
						},
					},
					AddressStatus: duckv1.AddressStatus{},
				},
			},
		},
		"has domain": {
			address: &duckv1.Addressable{
				URL: &apis.URL{
					Scheme: "http",
					Host:   "test-domain",
				},
			},
			want: &ChannelStatus{
				ChannelableStatus: v1beta1.ChannelableStatus{
					AddressStatus: duckv1.AddressStatus{
						Address: &duckv1.Addressable{
							URL: &apis.URL{
								Scheme: "http",
								Host:   "test-domain",
							},
						},
					},
					Status: duckv1.Status{
						Conditions: []apis.Condition{
							{
								Type:   ChannelConditionAddressable,
								Status: corev1.ConditionTrue,
							}, {
								// Note: Ready is here because when the condition
								// is marked True, duck automatically sets Ready to
								// Unknown because of missing ChannelConditionBackingChannelReady.
								Type:   ChannelConditionReady,
								Status: corev1.ConditionUnknown,
							}},
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			cs := &ChannelStatus{}
			cs.SetAddress(tc.address)
			ignore := cmpopts.IgnoreFields(
				apis.Condition{},
				"LastTransitionTime", "Message", "Reason", "Severity")
			if diff := cmp.Diff(tc.want, cs, ignore); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}

func TestChannelPropagateStatuses(t *testing.T) {
	testCases := map[string]struct {
		channelableStatus   *v1beta1.ChannelableStatus
		wantConditionStatus corev1.ConditionStatus
	}{
		"address set": {
			channelableStatus: &v1beta1.ChannelableStatus{
				AddressStatus: duckv1.AddressStatus{
					Address: validAddress,
				},
			},
			wantConditionStatus: corev1.ConditionUnknown,
		},
		"address not set": {
			channelableStatus: &v1beta1.ChannelableStatus{
				AddressStatus: duckv1.AddressStatus{
					Address: &duckv1.Addressable{},
				},
			},
			wantConditionStatus: corev1.ConditionFalse,
		},
		"url nil": {
			channelableStatus: &v1beta1.ChannelableStatus{
				AddressStatus: duckv1.AddressStatus{
					Address: nil,
				},
			},
			wantConditionStatus: corev1.ConditionFalse,
		},
		"all set": {
			channelableStatus: &v1beta1.ChannelableStatus{
				AddressStatus: duckv1.AddressStatus{
					Address: validAddress,
				},
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   apis.ConditionReady,
						Status: corev1.ConditionTrue,
					}},
				},
			},
			wantConditionStatus: corev1.ConditionTrue,
		},
		"backing channel with unknown status": {
			channelableStatus: &v1beta1.ChannelableStatus{
				AddressStatus: duckv1.AddressStatus{
					Address: validAddress,
				},
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   apis.ConditionReady,
						Status: corev1.ConditionUnknown,
					}},
				},
			},
			wantConditionStatus: corev1.ConditionUnknown,
		},
		"no condition ready in backing channel": {
			channelableStatus: &v1beta1.ChannelableStatus{
				AddressStatus: duckv1.AddressStatus{
					Address: validAddress,
				},
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}},
				},
			},
			wantConditionStatus: corev1.ConditionUnknown,
		},
		"test subscribableTypeStatus is set": {
			channelableStatus: &v1beta1.ChannelableStatus{
				SubscribableStatus: v1beta1.SubscribableStatus{
					// Populate ALL fields
					Subscribers: []v1beta1.SubscriberStatus{{
						UID:                "2f9b5e8e-deb6-11e8-9f32-f2801f1b9fd1",
						ObservedGeneration: 1,
						Ready:              corev1.ConditionTrue,
						Message:            "Some message",
					}, {
						UID:                "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
						ObservedGeneration: 2,
						Ready:              corev1.ConditionFalse,
						Message:            "Some message",
					}},
				},
			},
			wantConditionStatus: corev1.ConditionFalse,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			cs := &ChannelStatus{}
			cs.PropagateStatuses(tc.channelableStatus)
			got := cs.GetTopLevelCondition().Status
			if tc.wantConditionStatus != got {
				t.Errorf("unexpected readiness: want %v, got %v", tc.wantConditionStatus, got)
			}
		})
	}
}
