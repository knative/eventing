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

	"github.com/google/go-cmp/cmp"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var (
	brokerConditionReady = duckv1alpha1.Condition{
		Type:   BrokerConditionReady,
		Status: corev1.ConditionTrue,
	}

	brokerConditionIngress = duckv1alpha1.Condition{
		Type:   BrokerConditionIngress,
		Status: corev1.ConditionTrue,
	}

	brokerConditionChannel = duckv1alpha1.Condition{
		Type:   BrokerConditionChannel,
		Status: corev1.ConditionTrue,
	}

	brokerConditionFilter = duckv1alpha1.Condition{
		Type:   BrokerConditionFilter,
		Status: corev1.ConditionTrue,
	}

	brokerConditionAddressable = duckv1alpha1.Condition{
		Type:   BrokerConditionAddressable,
		Status: corev1.ConditionFalse,
	}
)

func TestBrokerGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		bs        *BrokerStatus
		condQuery duckv1alpha1.ConditionType
		want      *duckv1alpha1.Condition
	}{{
		name: "single condition",
		bs: &BrokerStatus{
			Conditions: []duckv1alpha1.Condition{
				brokerConditionReady,
			},
		},
		condQuery: duckv1alpha1.ConditionReady,
		want:      &brokerConditionReady,
	}, {
		name: "multiple conditions",
		bs: &BrokerStatus{
			Conditions: []duckv1alpha1.Condition{
				brokerConditionIngress,
				brokerConditionChannel,
				brokerConditionFilter,
			},
		},
		condQuery: BrokerConditionFilter,
		want:      &brokerConditionFilter,
	}, {
		name: "multiple conditions, condition false",
		bs: &BrokerStatus{
			Conditions: []duckv1alpha1.Condition{
				brokerConditionChannel,
				brokerConditionFilter,
				brokerConditionAddressable,
			},
		},
		condQuery: BrokerConditionAddressable,
		want:      &brokerConditionAddressable,
	}, {
		name: "unknown condition",
		bs: &BrokerStatus{
			Conditions: []duckv1alpha1.Condition{
				brokerConditionAddressable,
				brokerConditionReady,
			},
		},
		condQuery: duckv1alpha1.ConditionType("foo"),
		want:      nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.bs.GetCondition(test.condQuery)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}

func TestBrokerInitializeConditions(t *testing.T) {
	tests := []struct {
		name string
		bs   *BrokerStatus
		want *BrokerStatus
	}{{
		name: "empty",
		bs:   &BrokerStatus{},
		want: &BrokerStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   BrokerConditionAddressable,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionChannel,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionFilter,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionIngress,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionReady,
				Status: corev1.ConditionUnknown,
			}},
		},
	}, {
		name: "one false",
		bs: &BrokerStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   BrokerConditionChannel,
				Status: corev1.ConditionFalse,
			}},
		},
		want: &BrokerStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   BrokerConditionAddressable,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionChannel,
				Status: corev1.ConditionFalse,
			}, {
				Type:   BrokerConditionFilter,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionIngress,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionReady,
				Status: corev1.ConditionUnknown,
			}},
		},
	}, {
		name: "one true",
		bs: &BrokerStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   BrokerConditionFilter,
				Status: corev1.ConditionTrue,
			}},
		},
		want: &BrokerStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   BrokerConditionAddressable,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionChannel,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionFilter,
				Status: corev1.ConditionTrue,
			}, {
				Type:   BrokerConditionIngress,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionReady,
				Status: corev1.ConditionUnknown,
			}},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.bs.InitializeConditions()
			if diff := cmp.Diff(test.want, test.bs, ignoreAllButTypeAndStatus); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}

func TestBrokerIsReady(t *testing.T) {
	tests := []struct {
		name             string
		markChannelReady bool
		markFilterReady  bool
		markIngressReady bool
		address          string
		wantReady        bool
	}{{
		name:             "all happy",
		markChannelReady: true,
		markFilterReady:  true,
		markIngressReady: true,
		address:          "hostname",
		wantReady:        true,
	}, {
		name:             "channel sad",
		markChannelReady: false,
		markFilterReady:  true,
		markIngressReady: true,
		address:          "hostname",
		wantReady:        false,
	}, {
		name:             "filter sad",
		markChannelReady: true,
		markFilterReady:  false,
		markIngressReady: true,
		address:          "hostname",
		wantReady:        false,
	}, {
		name:             "ingress sad",
		markChannelReady: true,
		markFilterReady:  true,
		markIngressReady: false,
		address:          "hostname",
		wantReady:        false,
	}, {
		name:             "addressable sad",
		markChannelReady: true,
		markFilterReady:  true,
		markIngressReady: true,
		address:          "",
		wantReady:        false,
	}, {
		name:             "all sad",
		markChannelReady: false,
		markFilterReady:  false,
		markIngressReady: false,
		address:          "",
		wantReady:        false,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ts := &BrokerStatus{}
			if test.markChannelReady {
				ts.MarkChannelReady()
			}
			if test.markFilterReady {
				ts.MarkFilterReady()
			}
			if test.markIngressReady {
				ts.MarkIngressReady()
			}
			ts.SetAddress(test.address)
			got := ts.IsReady()
			if test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}
		})
	}
}
