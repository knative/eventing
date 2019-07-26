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
	"github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
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
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{
					condReady,
				},
			},
		},
		condQuery: apis.ConditionReady,
		want:      &condReady,
	}, {
		name: "unknown condition",
		cs: &ChannelStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{
					condReady,
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
			Status: duckv1beta1.Status{
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
	}, {
		name: "one false",
		cs: &ChannelStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   ChannelConditionAddressable,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &ChannelStatus{
			Status: duckv1beta1.Status{
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
	}, {
		name: "one true",
		cs: &ChannelStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   ChannelConditionBackingChannelReady,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		want: &ChannelStatus{
			Status: duckv1beta1.Status{
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

func TestChannelIsReady(t *testing.T) {
	tests := []struct {
		name                    string
		setAddress              bool
		markBackingChannelReady bool
		wantReady               bool
	}{{
		name:                    "all happy",
		setAddress:              true,
		markBackingChannelReady: true,
		wantReady:               true,
	}, {
		name:                    "address not set",
		setAddress:              false,
		markBackingChannelReady: true,
		wantReady:               false,
	}, {
		name:                    "backing channel not ready",
		setAddress:              true,
		markBackingChannelReady: false,
		wantReady:               false,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cs := &ChannelStatus{}
			cs.InitializeConditions()
			if test.setAddress {
				cs.SetAddress(&duckv1alpha1.Addressable{
					Addressable: duckv1beta1.Addressable{
						URL: &apis.URL{
							Scheme: "http",
							Host:   "test-domain",
						},
					},
				})
			}
			if test.markBackingChannelReady {
				cs.MarkBackingChannelReady()
			} else {
				cs.MarkBackingChannelFailed("ChannelFailure", "testing")
			}
			got := cs.IsReady()
			if test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}
		})
	}
}

func TestChannelSetAddressable(t *testing.T) {
	testCases := map[string]struct {
		address *duckv1alpha1.Addressable
		want    *ChannelStatus
	}{
		"nil url": {
			want: &ChannelStatus{
				Status: duckv1beta1.Status{
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
				AddressStatus: duckv1alpha1.AddressStatus{Address: &duckv1alpha1.Addressable{}},
			},
		},
		"has domain": {
			address: &duckv1alpha1.Addressable{
				Addressable: duckv1beta1.Addressable{
					URL: &apis.URL{
						Scheme: "http",
						Host:   "test-domain",
					},
				},
				Hostname: "test-domain",
			},
			want: &ChannelStatus{
				AddressStatus: duckv1alpha1.AddressStatus{
					Address: &duckv1alpha1.Addressable{
						Addressable: duckv1beta1.Addressable{
							URL: &apis.URL{
								Scheme: "http",
								Host:   "test-domain",
							},
						},
						Hostname: "test-domain",
					},
				},
				Status: duckv1beta1.Status{
					Conditions: []apis.Condition{
						{
							Type:   ChannelConditionAddressable,
							Status: corev1.ConditionTrue,
						}},
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
		channelableStatus *v1alpha1.ChannelableStatus
		wantReady         bool
	}{
		"address set": {
			channelableStatus: &v1alpha1.ChannelableStatus{
				AddressStatus: duckv1alpha1.AddressStatus{
					Address: &duckv1alpha1.Addressable{
						Addressable: duckv1beta1.Addressable{
							URL: &apis.URL{
								Scheme: "http",
								Host:   "test-domain",
							},
						},
						Hostname: "test-domain",
					},
				},
			},
			wantReady: false,
		},
		"address not set": {
			channelableStatus: &v1alpha1.ChannelableStatus{
				AddressStatus: duckv1alpha1.AddressStatus{
					Address: &duckv1alpha1.Addressable{},
				},
			},
			wantReady: false,
		},
		"url not set": {
			channelableStatus: &v1alpha1.ChannelableStatus{
				AddressStatus: duckv1alpha1.AddressStatus{
					Address: &duckv1alpha1.Addressable{
						Addressable: duckv1beta1.Addressable{},
						Hostname:    "test-domain",
					},
				},
			},
			wantReady: false,
		},
		"all set": {
			channelableStatus: &v1alpha1.ChannelableStatus{
				AddressStatus: duckv1alpha1.AddressStatus{
					Address: &duckv1alpha1.Addressable{
						Addressable: duckv1beta1.Addressable{
							URL: &apis.URL{
								Scheme: "http",
								Host:   "test-domain",
							},
						},
						Hostname: "test-domain",
					},
				},
				Status: duckv1beta1.Status{
					Conditions: []apis.Condition{{
						Type:   apis.ConditionReady,
						Status: corev1.ConditionTrue,
					}},
				},
			},
			wantReady: true,
		},
		"backing channel not ready": {
			channelableStatus: &v1alpha1.ChannelableStatus{
				AddressStatus: duckv1alpha1.AddressStatus{
					Address: &duckv1alpha1.Addressable{
						Addressable: duckv1beta1.Addressable{
							URL: &apis.URL{
								Scheme: "http",
								Host:   "test-domain",
							},
						},
						Hostname: "test-domain",
					},
				},
				Status: duckv1beta1.Status{
					Conditions: []apis.Condition{{
						Type:   apis.ConditionReady,
						Status: corev1.ConditionUnknown,
					}},
				},
			},
			wantReady: false,
		},
		"no condition ready in backing channel": {
			channelableStatus: &v1alpha1.ChannelableStatus{
				AddressStatus: duckv1alpha1.AddressStatus{
					Address: &duckv1alpha1.Addressable{
						Addressable: duckv1beta1.Addressable{
							URL: &apis.URL{
								Scheme: "http",
								Host:   "test-domain",
							},
						},
						Hostname: "test-domain",
					},
				},
				Status: duckv1beta1.Status{
					Conditions: []apis.Condition{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}},
				},
			},
			wantReady: false,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			cs := &ChannelStatus{}
			cs.PropagateStatuses(tc.channelableStatus)
			got := cs.IsReady()
			if tc.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", tc.wantReady, got)
			}
		})
	}
}
