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

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/eventing"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	trueVal  = true
	falseVal = false
)

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

	url, _ = apis.ParseURL("http://example.com")
)

func TestBrokerGetConditionSet(t *testing.T) {

	customCondition := apis.NewLivingConditionSet(
		apis.ConditionReady,
		"ConditionGolangReady",
	)
	brokerClass := "Golang"

	tt := []struct {
		name                 string
		broker               Broker
		expectedConditionSet apis.ConditionSet
	}{
		{
			name:                 "default condition set",
			broker:               Broker{},
			expectedConditionSet: brokerCondSet,
		},
		{
			name: "custom condition set",
			broker: Broker{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						eventing.BrokerClassKey: brokerClass,
					},
				},
			},
			expectedConditionSet: customCondition,
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			defer RegisterAlternateBrokerConditionSet(brokerCondSet) // reset to default condition set

			RegisterAlternateBrokerConditionSet(tc.expectedConditionSet)

			if diff := cmp.Diff(tc.expectedConditionSet, tc.broker.GetConditionSet(), cmp.AllowUnexported(apis.ConditionSet{})); diff != "" {
				t.Error("unexpected conditions (-want, +got)", diff)
			}
			if diff := cmp.Diff(tc.expectedConditionSet, tc.broker.Status.GetConditionSet(), cmp.AllowUnexported(apis.ConditionSet{})); diff != "" {
				t.Error("unexpected conditions (-want, +got)", diff)
			}
		})
	}
}

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
				t.Error("unexpected condition (-want, +got) =", diff)
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
					Type:   BrokerConditionDeadLetterSinkResolved,
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
					Type:   BrokerConditionDeadLetterSinkResolved,
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
					Type:   BrokerConditionDeadLetterSinkResolved,
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
		}}, {
		name: "default ready status without defined DLS",
		bs:   TestHelper.ReadyBrokerStatusWithoutDLS(),
		want: &BrokerStatus{
			Address: duckv1.Addressable{URL: url},
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   BrokerConditionAddressable,
					Status: corev1.ConditionTrue,
				}, {
					Type:   BrokerConditionDeadLetterSinkResolved,
					Status: corev1.ConditionTrue,
				}, {
					Type:   BrokerConditionFilter,
					Status: corev1.ConditionTrue,
				}, {
					Type:   BrokerConditionIngress,
					Status: corev1.ConditionTrue,
				}, {
					Type:   BrokerConditionReady,
					Status: corev1.ConditionTrue,
				}, {
					Type:   BrokerConditionTriggerChannel,
					Status: corev1.ConditionTrue,
				}},
			},
		}}, {
		name: "broker ready condition",
		bs: &BrokerStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{*TestHelper.ReadyBrokerCondition()},
			},
		},
		want: &BrokerStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   BrokerConditionAddressable,
					Status: corev1.ConditionTrue,
				}, {
					Type:   BrokerConditionDeadLetterSinkResolved,
					Status: corev1.ConditionTrue,
				}, {
					Type:   BrokerConditionFilter,
					Status: corev1.ConditionTrue,
				}, {
					Type:   BrokerConditionIngress,
					Status: corev1.ConditionTrue,
				}, {
					Type:   BrokerConditionReady,
					Status: corev1.ConditionTrue,
				}, {
					Type:   BrokerConditionTriggerChannel,
					Status: corev1.ConditionTrue,
				}},
			},
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.bs.InitializeConditions()
			if diff := cmp.Diff(test.want, test.bs, ignoreAllButTypeAndStatus); diff != "" {
				t.Error("unexpected conditions (-want, +got) =", diff)
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
		markDLSResolved              *bool
		markAddressable              *bool
		address                      *apis.URL
		markIngressSubscriptionOwned bool
		markIngressSubscriptionReady *bool
		wantReady                    bool
	}{{
		name:                         "all happy",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		markDLSResolved:              &trueVal,
		address:                      &apis.URL{Scheme: "http", Host: "hostname"},
		markIngressSubscriptionOwned: true,
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    true,
	}, {
		name:                         "all happy - deprecated",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		markDLSResolved:              &trueVal,
		address:                      &apis.URL{Scheme: "http", Host: "hostname"},
		markIngressSubscriptionOwned: true,
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    true,
	}, {
		name:                         "ingress sad",
		markIngressReady:             &falseVal,
		markTriggerChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		markDLSResolved:              &trueVal,
		address:                      &apis.URL{Scheme: "http", Host: "hostname"},
		markIngressSubscriptionOwned: true,
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    false,
	}, {
		name:                         "trigger channel sad",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &falseVal,
		markFilterReady:              &trueVal,
		markDLSResolved:              &trueVal,
		address:                      &apis.URL{Scheme: "http", Host: "hostname"},
		markIngressSubscriptionOwned: true,
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    false,
	}, {
		name:                         "filter sad",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markFilterReady:              &falseVal,
		markDLSResolved:              &trueVal,
		address:                      &apis.URL{Scheme: "http", Host: "hostname"},
		markIngressSubscriptionOwned: true,
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    false,
	}, {
		name:                         "addressable sad",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		markDLSResolved:              &trueVal,
		address:                      nil,
		markIngressSubscriptionOwned: true,
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    false,
	}, {
		name:                         "addressable unknown",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		markDLSResolved:              &trueVal,
		markAddressable:              nil,
		address:                      nil,
		markIngressSubscriptionOwned: true,
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    false,
	}, {
		name:                         "dls sad",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		markDLSResolved:              &falseVal,
		address:                      &apis.URL{Scheme: "http", Host: "hostname"},
		markIngressSubscriptionOwned: true,
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    false,
	}, {
		name:                         "dls not configured",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		markDLSResolved:              nil,
		address:                      &apis.URL{Scheme: "http", Host: "hostname"},
		markIngressSubscriptionOwned: true,
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    true,
	}, {
		name:                         "all sad",
		markIngressReady:             &falseVal,
		markTriggerChannelReady:      &falseVal,
		markFilterReady:              &falseVal,
		markDLSResolved:              &falseVal,
		address:                      nil,
		markIngressSubscriptionOwned: true,
		markIngressSubscriptionReady: &falseVal,
		wantReady:                    false,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bs := BrokerStatus{}
			if test.markIngressReady != nil {
				var ep *corev1.Endpoints
				if *test.markIngressReady {
					ep = TestHelper.AvailableEndpoints()
				} else {
					ep = TestHelper.UnavailableEndpoints()
				}
				bs.PropagateIngressAvailability(ep)
			}
			if test.markTriggerChannelReady != nil {
				var c *eventingduckv1.ChannelableStatus
				if *test.markTriggerChannelReady {
					c = TestHelper.ReadyChannelStatus()
				} else {
					c = TestHelper.NotReadyChannelStatus()
				}
				bs.PropagateTriggerChannelReadiness(c)
			}

			if test.markDLSResolved == &trueVal {
				url, _ := apis.ParseURL("test")
				bs.MarkDeadLetterSinkResolvedSucceeded(url)
			} else if test.markDLSResolved == &falseVal {
				bs.MarkDeadLetterSinkResolvedFailed("Unable to get the dead letter sink's URI", "DLS reference not found")
			} else {
				bs.MarkDeadLetterSinkNotConfigured()
			}

			if test.markFilterReady != nil {
				var ep *corev1.Endpoints
				if *test.markFilterReady {
					ep = TestHelper.AvailableEndpoints()
				} else {
					ep = TestHelper.UnavailableEndpoints()
				}
				bs.PropagateFilterAvailability(ep)
			}

			if test.markAddressable == nil && test.address == nil {
				bs.MarkBrokerAddressableUnknown("", "")
			}
			bs.SetAddress(test.address)

			b := Broker{Status: bs}
			got := b.IsReady()
			if test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}

			b.Generation = 1
			b.Status.ObservedGeneration = 2
			if b.IsReady() {
				t.Error("Expected IsReady() to be false when Generation != ObservedGeneration")
			}
		})
	}
}
