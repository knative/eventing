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

	brokerConditionTriggerChannel = duckv1alpha1.Condition{
		Type:   BrokerConditionTriggerChannel,
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
				brokerConditionTriggerChannel,
				brokerConditionFilter,
			},
		},
		condQuery: BrokerConditionFilter,
		want:      &brokerConditionFilter,
	}, {
		name: "multiple conditions, condition false",
		bs: &BrokerStatus{
			Conditions: []duckv1alpha1.Condition{
				brokerConditionTriggerChannel,
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
				Type:   BrokerConditionFilter,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionIngressChannel,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionIngress,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionIngressSubscription,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionReady,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionTriggerChannel,
				Status: corev1.ConditionUnknown,
			}},
		},
	}, {
		name: "one false",
		bs: &BrokerStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   BrokerConditionTriggerChannel,
				Status: corev1.ConditionFalse,
			}},
		},
		want: &BrokerStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   BrokerConditionAddressable,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionFilter,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionIngressChannel,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionIngress,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionIngressSubscription,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionReady,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionTriggerChannel,
				Status: corev1.ConditionFalse,
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
				Type:   BrokerConditionFilter,
				Status: corev1.ConditionTrue,
			}, {
				Type:   BrokerConditionIngressChannel,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionIngress,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionIngressSubscription,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionReady,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   BrokerConditionTriggerChannel,
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
		name                         string
		markIngressReady             bool
		markTriggerChannelReady      bool
		markIngressChannelReady      bool
		markFilterReady              bool
		address                      string
		markIngressSubscriptionReady bool
		wantReady                    bool
	}{{
		name:                         "all happy",
		markIngressReady:             true,
		markTriggerChannelReady:      true,
		markIngressChannelReady:      true,
		markFilterReady:              true,
		address:                      "hostname",
		markIngressSubscriptionReady: true,
		wantReady:                    true,
	}, {
		name:                         "ingress sad",
		markIngressReady:             false,
		markTriggerChannelReady:      true,
		markIngressChannelReady:      true,
		markFilterReady:              true,
		address:                      "hostname",
		markIngressSubscriptionReady: true,
		wantReady:                    false,
	}, {
		name:                         "trigger channel sad",
		markIngressReady:             true,
		markTriggerChannelReady:      false,
		markIngressChannelReady:      true,
		markFilterReady:              true,
		address:                      "hostname",
		markIngressSubscriptionReady: true,
		wantReady:                    false,
	}, {
		name:                         "ingress channel sad",
		markIngressReady:             true,
		markTriggerChannelReady:      true,
		markIngressChannelReady:      false,
		markFilterReady:              true,
		address:                      "hostname",
		markIngressSubscriptionReady: true,
		wantReady:                    false,
	}, {
		name:                         "filter sad",
		markIngressReady:             true,
		markTriggerChannelReady:      true,
		markIngressChannelReady:      true,
		markFilterReady:              false,
		address:                      "hostname",
		markIngressSubscriptionReady: true,
		wantReady:                    false,
	}, {
		name:                         "addressable sad",
		markIngressReady:             true,
		markTriggerChannelReady:      true,
		markIngressChannelReady:      true,
		markFilterReady:              true,
		address:                      "",
		markIngressSubscriptionReady: true,
		wantReady:                    false,
	}, {
		name:                         "ingress subscription sad",
		markIngressReady:             true,
		markTriggerChannelReady:      true,
		markIngressChannelReady:      true,
		markFilterReady:              true,
		address:                      "hostname",
		markIngressSubscriptionReady: false,
		wantReady:                    false,
	}, {
		name:                         "all sad",
		markIngressReady:             false,
		markTriggerChannelReady:      false,
		markIngressChannelReady:      true,
		markFilterReady:              false,
		address:                      "",
		markIngressSubscriptionReady: true,
		wantReady:                    false,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ts := &BrokerStatus{}
			if test.markIngressReady {
				ts.MarkIngressReady()
			}
			if test.markTriggerChannelReady {
				ts.MarkTriggerChannelReady()
			}
			if test.markIngressChannelReady {
				ts.MarkIngressChannelReady()
			}
			if test.markFilterReady {
				ts.MarkFilterReady()
			}
			ts.SetAddress(test.address)
			if test.markIngressSubscriptionReady {
				ts.MarkIngressSubscriptionReady()
			}
			got := ts.IsReady()
			if test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}
		})
	}
}
