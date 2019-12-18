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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	trueVal  = true
	falseVal = false
)

var ignoreAllButTypeAndStatus = cmpopts.IgnoreFields(
	apis.Condition{},
	"LastTransitionTime", "Message", "Reason", "Severity")

var (
	brokerConditionReady = apis.Condition{
		Type:   BrokerConditionReady,
		Status: corev1.ConditionTrue,
	}

	brokerConditionIngress = apis.Condition{
		Type:   BrokerConditionIngress,
		Status: corev1.ConditionTrue,
	}

	brokerConditionTriggerChannel = apis.Condition{
		Type:   BrokerConditionTriggerChannel,
		Status: corev1.ConditionTrue,
	}

	brokerConditionFilter = apis.Condition{
		Type:   BrokerConditionFilter,
		Status: corev1.ConditionTrue,
	}

	brokerConditionAddressable = apis.Condition{
		Type:   BrokerConditionAddressable,
		Status: corev1.ConditionFalse,
	}
)

func TestBrokerGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		bs        *BrokerStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name: "single condition",
		bs: &BrokerStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					brokerConditionReady,
				},
			},
		},
		condQuery: apis.ConditionReady,
		want:      &brokerConditionReady,
	}, {
		name: "multiple conditions",
		bs: &BrokerStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					brokerConditionIngress,
					brokerConditionTriggerChannel,
					brokerConditionFilter,
				},
			},
		},
		condQuery: BrokerConditionFilter,
		want:      &brokerConditionFilter,
	}, {
		name: "multiple conditions, condition false",
		bs: &BrokerStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					brokerConditionTriggerChannel,
					brokerConditionFilter,
					brokerConditionAddressable,
				},
			},
		},
		condQuery: BrokerConditionAddressable,
		want:      &brokerConditionAddressable,
	}, {
		name: "unknown condition",
		bs: &BrokerStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					brokerConditionAddressable,
					brokerConditionReady,
				},
			},
		},
		condQuery: apis.ConditionType("foo"),
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
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   BrokerConditionAddressable,
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
				}, {
					Type:   BrokerConditionTriggerChannel,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one false",
		bs: &BrokerStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   BrokerConditionTriggerChannel,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &BrokerStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   BrokerConditionAddressable,
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
				}, {
					Type:   BrokerConditionTriggerChannel,
					Status: corev1.ConditionFalse,
				}},
			},
		},
	}, {
		name: "one true",
		bs: &BrokerStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   BrokerConditionFilter,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		want: &BrokerStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   BrokerConditionAddressable,
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
				}, {
					Type:   BrokerConditionTriggerChannel,
					Status: corev1.ConditionUnknown,
				}},
			},
		}},
	}

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
		markIngressReady             *bool
		markTriggerChannelReady      *bool
		markFilterReady              *bool
		address                      *apis.URL
		markIngressSubscriptionOwned bool
		markIngressSubscriptionReady *bool
		wantReady                    bool
	}{{
		name:                         "all happy",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		address:                      &apis.URL{Scheme: "http", Host: "hostname"},
		markIngressSubscriptionOwned: true,
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    true,
	}, {
		name:                         "all happy - deprecated",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		address:                      &apis.URL{Scheme: "http", Host: "hostname"},
		markIngressSubscriptionOwned: true,
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    true,
	}, {
		name:                         "ingress sad",
		markIngressReady:             &falseVal,
		markTriggerChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		address:                      &apis.URL{Scheme: "http", Host: "hostname"},
		markIngressSubscriptionOwned: true,
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    false,
	}, {
		name:                         "trigger channel sad",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &falseVal,
		markFilterReady:              &trueVal,
		address:                      &apis.URL{Scheme: "http", Host: "hostname"},
		markIngressSubscriptionOwned: true,
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    false,
	}, {
		name:                         "filter sad",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markFilterReady:              &falseVal,
		address:                      &apis.URL{Scheme: "http", Host: "hostname"},
		markIngressSubscriptionOwned: true,
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    false,
	}, {
		name:                         "addressable sad",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		address:                      nil,
		markIngressSubscriptionOwned: true,
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    false,
	}, {
		name:                         "all sad",
		markIngressReady:             &falseVal,
		markTriggerChannelReady:      &falseVal,
		markFilterReady:              &falseVal,
		address:                      nil,
		markIngressSubscriptionOwned: true,
		markIngressSubscriptionReady: &falseVal,
		wantReady:                    false,
	}}

	for _, test := range tests {
		testName := fmt.Sprintf("%s", test.name)
		//			t.Run(test.name, func(t *testing.T) {
		t.Run(testName, func(t *testing.T) {
			bs := &BrokerStatus{}
			if test.markIngressReady != nil {
				var d *v1.Deployment
				if *test.markIngressReady {
					d = TestHelper.AvailableDeployment()
				} else {
					d = TestHelper.UnavailableDeployment()
				}
				bs.PropagateIngressDeploymentAvailability(d)
			}
			if test.markTriggerChannelReady != nil {
				var c *duckv1alpha1.ChannelableStatus
				if *test.markTriggerChannelReady {
					c = TestHelper.ReadyChannelStatus()
				} else {
					c = TestHelper.NotReadyChannelStatus()
				}
				bs.PropagateTriggerChannelReadiness(c)
			}
			if test.markFilterReady != nil {
				var d *v1.Deployment
				if *test.markFilterReady {
					d = TestHelper.AvailableDeployment()
				} else {
					d = TestHelper.UnavailableDeployment()
				}
				bs.PropagateFilterDeploymentAvailability(d)
			}
			bs.SetAddress(test.address)

			got := bs.IsReady()
			if test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}

		})
	}
}
