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
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var (
	trueVal  = true
	falseVal = false
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
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{
					brokerConditionReady,
				},
			},
		},
		condQuery: duckv1alpha1.ConditionReady,
		want:      &brokerConditionReady,
	}, {
		name: "multiple conditions",
		bs: &BrokerStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{
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
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{
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
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{
					brokerConditionAddressable,
					brokerConditionReady,
				},
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
			Status: duckv1alpha1.Status{
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
		},
	}, {
		name: "one false",
		bs: &BrokerStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
					Type:   BrokerConditionTriggerChannel,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &BrokerStatus{
			Status: duckv1alpha1.Status{
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
		},
	}, {
		name: "one true",
		bs: &BrokerStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
					Type:   BrokerConditionFilter,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		want: &BrokerStatus{
			Status: duckv1alpha1.Status{
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
		markIngressChannelReady      *bool
		markFilterReady              *bool
		address                      string
		markIngressSubscriptionReady *bool
		wantReady                    bool
	}{{
		name:                         "all happy",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markIngressChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		address:                      "hostname",
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    true,
	}, {
		name:                         "ingress sad",
		markIngressReady:             &falseVal,
		markTriggerChannelReady:      &trueVal,
		markIngressChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		address:                      "hostname",
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    false,
	}, {
		name:                         "trigger channel sad",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &falseVal,
		markIngressChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		address:                      "hostname",
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    false,
	}, {
		name:                         "ingress channel sad",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markIngressChannelReady:      &falseVal,
		markFilterReady:              &trueVal,
		address:                      "hostname",
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    false,
	}, {
		name:                         "filter sad",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markIngressChannelReady:      &trueVal,
		markFilterReady:              &falseVal,
		address:                      "hostname",
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    false,
	}, {
		name:                         "addressable sad",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markIngressChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		address:                      "",
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    false,
	}, {
		name:                         "ingress subscription sad",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markIngressChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		address:                      "hostname",
		markIngressSubscriptionReady: &falseVal,
		wantReady:                    false,
	}, {
		name:                         "all sad",
		markIngressReady:             &falseVal,
		markTriggerChannelReady:      &falseVal,
		markIngressChannelReady:      &trueVal,
		markFilterReady:              &falseVal,
		address:                      "",
		markIngressSubscriptionReady: &trueVal,
		wantReady:                    false,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bs := &BrokerStatus{}
			if test.markIngressReady != nil {
				var d *v1.Deployment
				if *test.markIngressReady {
					d = availableDeployment()
				} else {
					d = unavailableDeployment()
				}
				bs.PropagateIngressDeploymentAvailability(d)
			}
			if test.markTriggerChannelReady != nil {
				var c *ChannelStatus
				if *test.markTriggerChannelReady {
					c = readyChannelStatus()
				} else {
					c = notReadyChannelStatus()
				}
				bs.PropagateTriggerChannelReadiness(c)
			}
			if test.markIngressChannelReady != nil {
				var c *ChannelStatus
				if *test.markIngressChannelReady {
					c = readyChannelStatus()
				} else {
					c = notReadyChannelStatus()
				}
				bs.PropagateIngressChannelReadiness(c)
			}
			if test.markIngressSubscriptionReady != nil {
				var sub *SubscriptionStatus
				if *test.markIngressSubscriptionReady {
					sub = readySubscriptionStatus()
				} else {
					sub = notReadySubscriptionStatus()
				}
				bs.PropagateIngressSubscriptionReadiness(sub)
			}
			if test.markFilterReady != nil {
				var d *v1.Deployment
				if *test.markFilterReady {
					d = availableDeployment()
				} else {
					d = unavailableDeployment()
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

func unavailableDeployment() *v1.Deployment {
	d := &v1.Deployment{}
	d.Name = "unavailable"
	d.Status.Conditions = []v1.DeploymentCondition{
		{
			Type:   v1.DeploymentAvailable,
			Status: "False",
		},
	}
	return d
}

func availableDeployment() *v1.Deployment {
	d := unavailableDeployment()
	d.Name = "available"
	d.Status.Conditions = []v1.DeploymentCondition{
		{
			Type:   v1.DeploymentAvailable,
			Status: "True",
		},
	}
	return d
}

func readyChannelStatus() *ChannelStatus {
	cs := &ChannelStatus{}
	cs.MarkProvisionerInstalled()
	cs.MarkProvisioned()
	cs.SetAddress("foo")
	return cs
}

func notReadyChannelStatus() *ChannelStatus {
	cs := readyChannelStatus()
	cs.MarkNotProvisioned("foo", "bar")
	return cs
}

func readySubscriptionStatus() *SubscriptionStatus {
	ss := &SubscriptionStatus{}
	ss.MarkChannelReady()
	ss.MarkReferencesResolved()
	return ss
}

func notReadySubscriptionStatus() *SubscriptionStatus {
	ss := &SubscriptionStatus{}
	ss.MarkReferencesResolved()
	return ss
}
