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
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	duckv1alpha1 "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/apis/eventing"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	v1 "k8s.io/api/apps/v1"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
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
			Status: duckv1beta1.Status{
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
			Status: duckv1beta1.Status{
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
			Status: duckv1beta1.Status{
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
			Status: duckv1beta1.Status{
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
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
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
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   BrokerConditionTriggerChannel,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &BrokerStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
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
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   BrokerConditionFilter,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		want: &BrokerStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
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
		address                      *apis.URL
		markIngressSubscriptionReady *bool
		markDeprecated               bool
		wantReady                    bool
	}{{
		name:                         "all happy",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markIngressChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		address:                      &apis.URL{Scheme: "http", Host: "hostname"},
		markIngressSubscriptionReady: &trueVal,
		markDeprecated:               false,
		wantReady:                    true,
	}, {
		name:                         "all happy - deprecated",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markIngressChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		address:                      &apis.URL{Scheme: "http", Host: "hostname"},
		markIngressSubscriptionReady: &trueVal,
		markDeprecated:               true,
		wantReady:                    true,
	}, {
		name:                         "ingress sad",
		markIngressReady:             &falseVal,
		markTriggerChannelReady:      &trueVal,
		markIngressChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		address:                      &apis.URL{Scheme: "http", Host: "hostname"},
		markIngressSubscriptionReady: &trueVal,
		markDeprecated:               false,
		wantReady:                    false,
	}, {
		name:                         "trigger channel sad",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &falseVal,
		markIngressChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		address:                      &apis.URL{Scheme: "http", Host: "hostname"},
		markIngressSubscriptionReady: &trueVal,
		markDeprecated:               false,
		wantReady:                    false,
	}, {
		name:                         "ingress channel sad",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markIngressChannelReady:      &falseVal,
		markFilterReady:              &trueVal,
		address:                      &apis.URL{Scheme: "http", Host: "hostname"},
		markIngressSubscriptionReady: &trueVal,
		markDeprecated:               false,
		wantReady:                    false,
	}, {
		name:                         "filter sad",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markIngressChannelReady:      &trueVal,
		markFilterReady:              &falseVal,
		address:                      &apis.URL{Scheme: "http", Host: "hostname"},
		markIngressSubscriptionReady: &trueVal,
		markDeprecated:               false,
		wantReady:                    false,
	}, {
		name:                         "addressable sad",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markIngressChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		address:                      nil,
		markIngressSubscriptionReady: &trueVal,
		markDeprecated:               false,
		wantReady:                    false,
	}, {
		name:                         "ingress subscription sad",
		markIngressReady:             &trueVal,
		markTriggerChannelReady:      &trueVal,
		markIngressChannelReady:      &trueVal,
		markFilterReady:              &trueVal,
		address:                      &apis.URL{Scheme: "http", Host: "hostname"},
		markIngressSubscriptionReady: &falseVal,
		markDeprecated:               false,
		wantReady:                    false,
	}, {
		name:                         "all sad",
		markIngressReady:             &falseVal,
		markTriggerChannelReady:      &falseVal,
		markIngressChannelReady:      &trueVal,
		markFilterReady:              &falseVal,
		address:                      nil,
		markIngressSubscriptionReady: &trueVal,
		markDeprecated:               false,
		wantReady:                    false,
	}}

	for _, test := range tests {
		for _, useCRDPropagation := range []bool{true, false} {
			testName := fmt.Sprintf("%s - using CRD %t", test.name, useCRDPropagation)
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
				if test.markTriggerChannelReady != nil && useCRDPropagation {
					var c *duckv1alpha1.ChannelableStatus
					if *test.markTriggerChannelReady {
						c = TestHelper.ReadyChannelStatusCRD()
					} else {
						c = TestHelper.NotReadyChannelStatusCRD()
					}
					bs.PropagateTriggerChannelReadinessCRD(c)
				}
				if test.markTriggerChannelReady != nil && !useCRDPropagation {
					var c *ChannelStatus
					if *test.markTriggerChannelReady {
						c = TestHelper.ReadyChannelStatus()
					} else {
						c = TestHelper.NotReadyChannelStatus()
					}
					bs.PropagateTriggerChannelReadiness(c)
				}
				if test.markIngressChannelReady != nil && useCRDPropagation {
					var c *duckv1alpha1.ChannelableStatus
					if *test.markIngressChannelReady {
						c = TestHelper.ReadyChannelStatusCRD()
					} else {
						c = TestHelper.NotReadyChannelStatusCRD()
					}
					bs.PropagateIngressChannelReadinessCRD(c)
				}
				if test.markIngressChannelReady != nil && !useCRDPropagation {
					var c *ChannelStatus
					if *test.markIngressChannelReady {
						c = TestHelper.ReadyChannelStatus()
					} else {
						c = TestHelper.NotReadyChannelStatus()
					}
					bs.PropagateIngressChannelReadiness(c)
				}
				if test.markIngressSubscriptionReady != nil {
					var sub *SubscriptionStatus
					if *test.markIngressSubscriptionReady {
						sub = TestHelper.ReadySubscriptionStatus()
					} else {
						sub = TestHelper.NotReadySubscriptionStatus()
					}
					bs.PropagateIngressSubscriptionReadiness(sub)
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
				if test.markDeprecated {
					bs.MarkDeprecated("TestReason", "Test Message For Deprecation")
				}
				bs.SetAddress(test.address)

				got := bs.IsReady()
				if test.wantReady != got {
					t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
				}

			})
		}
	}
}

func TestBrokerAnnotateUserInfo(t *testing.T) {
	const (
		u1 = "oveja@knative.dev"
		u2 = "cabra@knative.dev"
		u3 = "vaca@knative.dev"
	)

	withUserAnns := func(creator, updater string, b *Broker) *Broker {
		a := b.GetAnnotations()
		if a == nil {
			a = map[string]string{}
			defer b.SetAnnotations(a)
		}

		a[eventing.CreatorAnnotation] = creator
		a[eventing.UpdaterAnnotation] = updater

		return b
	}

	tests := []struct {
		name       string
		user       string
		this       *Broker
		prev       *Broker
		wantedAnns map[string]string
	}{{
		"create new broker",
		u1,
		&Broker{},
		nil,
		map[string]string{
			eventing.CreatorAnnotation: u1,
			eventing.UpdaterAnnotation: u1,
		},
	}, {
		"update broker which has no annotations without diff",
		u1,
		&Broker{},
		&Broker{},
		map[string]string{},
	}, {
		"update broker which has annotations without diff",
		u2,
		withUserAnns(u1, u1, &Broker{}),
		withUserAnns(u1, u1, &Broker{}),
		map[string]string{
			eventing.CreatorAnnotation: u1,
			eventing.UpdaterAnnotation: u1,
		},
	}, {
		"update broker which has no annotations with diff",
		u2,
		&Broker{Spec: BrokerSpec{DeprecatedChannelTemplate: &ChannelSpec{DeprecatedGeneration: 1}}},
		&Broker{},
		map[string]string{
			eventing.UpdaterAnnotation: u2,
		}}, {
		"update broker which has annotations with diff",
		u3,
		withUserAnns(u1, u2, &Broker{Spec: BrokerSpec{DeprecatedChannelTemplate: &ChannelSpec{DeprecatedGeneration: 1}}}),
		withUserAnns(u1, u2, &Broker{}),
		map[string]string{
			eventing.CreatorAnnotation: u1,
			eventing.UpdaterAnnotation: u3,
		},
	}}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := apis.WithUserInfo(context.Background(), &authv1.UserInfo{
				Username: test.user,
			})
			if test.prev != nil {
				ctx = apis.WithinUpdate(ctx, test.prev)
			}
			test.this.SetDefaults(ctx)

			if got, want := test.this.GetAnnotations(), test.wantedAnns; !cmp.Equal(got, want) {
				t.Errorf("Annotations = %v, want: %v, diff (-got, +want): %s", got, want, cmp.Diff(got, want))
			}
		})
	}
}

func TestBrokerStatus_MarkDeprecated(t *testing.T) {
	testCases := map[string]struct {
		alreadyPresent bool
	}{
		"not present": {
			alreadyPresent: false,
		},
		"already present": {
			alreadyPresent: true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			bs := &BrokerStatus{}
			if tc.alreadyPresent {
				bs.MarkDeprecated("AlreadyPresent", "Already present.")
			}
			bs.MarkDeprecated("Test", "Test Message")
			if len(bs.Conditions) != 1 {
				t.Fatalf("Incorrect number of conditions. Expected 1, actually %v", bs)
			}

			expected := apis.Condition{
				Type:     "Deprecated",
				Reason:   "Test",
				Status:   corev1.ConditionTrue,
				Severity: apis.ConditionSeverityWarning,
				Message:  "Test Message",
			}
			if diff := cmp.Diff(expected, bs.Conditions[0], ignoreLastTransitionTime); diff != "" {
				t.Errorf("Condition incorrect (-want +got): %s", diff)
			}
		})
	}
}
