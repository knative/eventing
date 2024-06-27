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
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
)

var (
	condReady = apis.Condition{
		Type:   InMemoryChannelConditionReady,
		Status: corev1.ConditionTrue,
	}

	condDispatcherNotReady = apis.Condition{
		Type:   InMemoryChannelConditionDispatcherReady,
		Status: corev1.ConditionFalse,
	}

	trueVal  = true
	falseVal = false

	deploymentConditionReady = appsv1.DeploymentCondition{
		Type:   appsv1.DeploymentAvailable,
		Status: corev1.ConditionTrue,
	}

	deploymentConditionNotReady = appsv1.DeploymentCondition{
		Type:   appsv1.DeploymentAvailable,
		Status: corev1.ConditionFalse,
	}

	deploymentStatusReady    = &appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{deploymentConditionReady}}
	deploymentStatusNotReady = &appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{deploymentConditionNotReady}}

	ignoreAllButTypeAndStatus = cmpopts.IgnoreFields(
		apis.Condition{},
		"LastTransitionTime", "Message", "Reason", "Severity")
)

func TestInMemoryChannelGetConditionSet(t *testing.T) {
	r := &InMemoryChannel{}

	if got, want := r.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func TestInMemoryChannelGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		cs        *InMemoryChannelStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name: "single condition",
		cs: &InMemoryChannelStatus{
			ChannelableStatus: eventingduckv1.ChannelableStatus{
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
		cs: &InMemoryChannelStatus{
			ChannelableStatus: eventingduckv1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						condReady,
						condDispatcherNotReady,
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
				t.Error("unexpected condition (-want, +got) =", diff)
			}
		})
	}
}

func TestInMemoryChannelInitializeConditions(t *testing.T) {
	tests := []struct {
		name string
		cs   *InMemoryChannelStatus
		want *InMemoryChannelStatus
	}{{
		name: "empty",
		cs:   &InMemoryChannelStatus{},
		want: &InMemoryChannelStatus{
			ChannelableStatus: eventingduckv1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   InMemoryChannelConditionAddressable,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   InMemoryChannelConditionChannelServiceReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   InMemoryChannelConditionDeadLetterSinkResolved,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   InMemoryChannelConditionEndpointsReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   InMemoryChannelConditionEventPoliciesReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   InMemoryChannelConditionReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   InMemoryChannelConditionServiceReady,
						Status: corev1.ConditionUnknown,
					}},
				},
			},
		},
	}, {
		name: "one false",
		cs: &InMemoryChannelStatus{
			ChannelableStatus: eventingduckv1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   InMemoryChannelConditionDispatcherReady,
						Status: corev1.ConditionFalse,
					}},
				},
			},
		},
		want: &InMemoryChannelStatus{
			ChannelableStatus: eventingduckv1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   InMemoryChannelConditionAddressable,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   InMemoryChannelConditionChannelServiceReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   InMemoryChannelConditionDeadLetterSinkResolved,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   InMemoryChannelConditionDispatcherReady,
						Status: corev1.ConditionFalse,
					}, {
						Type:   InMemoryChannelConditionEndpointsReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   InMemoryChannelConditionEventPoliciesReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   InMemoryChannelConditionReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   InMemoryChannelConditionServiceReady,
						Status: corev1.ConditionUnknown,
					}},
				},
			},
		},
	}, {
		name: "one true",
		cs: &InMemoryChannelStatus{
			ChannelableStatus: eventingduckv1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   InMemoryChannelConditionDispatcherReady,
						Status: corev1.ConditionTrue,
					}},
				},
			},
		},
		want: &InMemoryChannelStatus{
			ChannelableStatus: eventingduckv1.ChannelableStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   InMemoryChannelConditionAddressable,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   InMemoryChannelConditionChannelServiceReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   InMemoryChannelConditionDeadLetterSinkResolved,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   InMemoryChannelConditionDispatcherReady,
						Status: corev1.ConditionTrue,
					}, {
						Type:   InMemoryChannelConditionEndpointsReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   InMemoryChannelConditionEventPoliciesReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   InMemoryChannelConditionReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   InMemoryChannelConditionServiceReady,
						Status: corev1.ConditionUnknown,
					}},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.cs.InitializeConditions()
			if diff := cmp.Diff(test.want, test.cs, ignoreAllButTypeAndStatus); diff != "" {
				t.Error("unexpected conditions (-want, +got) =", diff)
			}
		})
	}
}

func TestInMemoryChannelIsReady(t *testing.T) {
	tests := []struct {
		name                    string
		markServiceReady        bool
		markChannelServiceReady bool
		markEventPolicyReady    bool
		setAddress              bool
		markEndpointsReady      bool
		DLSResolved             *bool
		wantReady               bool
		dispatcherStatus        *appsv1.DeploymentStatus
	}{{
		name:                    "all happy",
		markServiceReady:        true,
		markChannelServiceReady: true,
		markEventPolicyReady:    true,
		markEndpointsReady:      true,
		dispatcherStatus:        deploymentStatusReady,
		setAddress:              true,
		wantReady:               true,
		DLSResolved:             &trueVal,
	}, {
		name:                    "service not ready",
		markServiceReady:        false,
		markChannelServiceReady: false,
		markEventPolicyReady:    true,
		markEndpointsReady:      true,
		dispatcherStatus:        deploymentStatusReady,
		setAddress:              true,
		wantReady:               false,
		DLSResolved:             &trueVal,
	}, {
		name:                    "endpoints not ready",
		markServiceReady:        true,
		markChannelServiceReady: false,
		markEventPolicyReady:    true,
		markEndpointsReady:      false,
		dispatcherStatus:        deploymentStatusReady,
		setAddress:              true,
		wantReady:               false,
		DLSResolved:             &trueVal,
	}, {
		name:                    "deployment not ready",
		markServiceReady:        true,
		markEndpointsReady:      true,
		markChannelServiceReady: false,
		markEventPolicyReady:    true,
		dispatcherStatus:        deploymentStatusNotReady,
		setAddress:              true,
		wantReady:               false,
		DLSResolved:             &trueVal,
	}, {
		name:                    "address not set",
		markServiceReady:        true,
		markChannelServiceReady: false,
		markEventPolicyReady:    true,
		markEndpointsReady:      true,
		dispatcherStatus:        deploymentStatusReady,
		setAddress:              false,
		wantReady:               false,
		DLSResolved:             &trueVal,
	}, {
		name:                    "channel service not ready",
		markServiceReady:        true,
		markChannelServiceReady: false,
		markEventPolicyReady:    true,
		markEndpointsReady:      true,
		dispatcherStatus:        deploymentStatusReady,
		setAddress:              true,
		wantReady:               false,
		DLSResolved:             &trueVal,
	}, {
		name:                    "dls sad",
		markServiceReady:        true,
		markChannelServiceReady: false,
		markEventPolicyReady:    true,
		markEndpointsReady:      true,
		dispatcherStatus:        deploymentStatusReady,
		setAddress:              true,
		wantReady:               false,
		DLSResolved:             &falseVal,
	}, {
		name:                    "dls not configured",
		markServiceReady:        true,
		markChannelServiceReady: false,
		markEventPolicyReady:    true,
		markEndpointsReady:      true,
		dispatcherStatus:        deploymentStatusReady,
		setAddress:              true,
		wantReady:               false,
		DLSResolved:             &trueVal,
	}, {
		name:                    "EventPolicy not ready",
		markServiceReady:        true,
		markChannelServiceReady: true,
		markEventPolicyReady:    false,
		markEndpointsReady:      true,
		dispatcherStatus:        deploymentStatusReady,
		setAddress:              true,
		wantReady:               false,
		DLSResolved:             &trueVal,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cs := InMemoryChannelStatus{}
			cs.InitializeConditions()
			if test.markServiceReady {
				cs.MarkServiceTrue()
			} else {
				cs.MarkServiceFailed("NotReadyService", "testing")
			}
			if test.markChannelServiceReady {
				cs.MarkChannelServiceTrue()
			} else {
				cs.MarkChannelServiceFailed("NotReadyChannelService", "testing")
			}
			if test.markEventPolicyReady {
				cs.MarkEventPoliciesTrue()
			} else {
				cs.MarkEndpointsFailed("NotReadyEventPolicy", "testing")
			}
			if test.setAddress {
				cs.SetAddress(&duckv1.Addressable{URL: &apis.URL{Scheme: "http", Host: "foo.bar"}})
			}
			if test.markEndpointsReady {
				cs.MarkEndpointsTrue()
			} else {
				cs.MarkEndpointsFailed("NotReadyEndpoints", "testing")
			}
			if test.dispatcherStatus != nil {
				cs.PropagateDispatcherStatus(test.dispatcherStatus)
			} else {
				cs.MarkDispatcherFailed("NotReadyDispatcher", "testing")
			}
			if test.DLSResolved == &trueVal {
				cs.MarkDeadLetterSinkResolvedSucceeded(eventingduckv1.DeliveryStatus{})
			} else if test.DLSResolved == &falseVal {
				cs.MarkDeadLetterSinkResolvedFailed("Unable to get the dead letter sink's URI", "DLS reference not found")
			} else {
				cs.MarkDeadLetterSinkNotConfigured()
			}
			imc := InMemoryChannel{Status: cs}
			got := imc.IsReady()
			if test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}

			imc.Generation = 1
			imc.Status.ObservedGeneration = 2
			if imc.IsReady() {
				t.Error("Expected IsReady() to be false when Generation != ObservedGeneration")
			}
		})
	}
}

func TestInMemoryChannelStatus_SetAddressable(t *testing.T) {
	testCases := map[string]struct {
		url  *apis.URL
		want *InMemoryChannelStatus
	}{
		"empty string": {
			want: &InMemoryChannelStatus{
				ChannelableStatus: eventingduckv1.ChannelableStatus{
					Status: duckv1.Status{
						Conditions: []apis.Condition{{
							Type:   InMemoryChannelConditionAddressable,
							Status: corev1.ConditionFalse,
						}, {
							// Note that Ready is here because when the condition is marked False, duck
							// automatically sets Ready to false.
							Type:   InMemoryChannelConditionReady,
							Status: corev1.ConditionFalse,
						}},
					},
					AddressStatus: duckv1.AddressStatus{Address: &duckv1.Addressable{}},
				},
			},
		},
		"has domain - unknown": {
			url: &apis.URL{Scheme: "http", Host: "test-domain"},
			want: &InMemoryChannelStatus{
				ChannelableStatus: eventingduckv1.ChannelableStatus{
					AddressStatus: duckv1.AddressStatus{
						Address: &duckv1.Addressable{
							Name: pointer.String("http"),
							URL: &apis.URL{
								Scheme: "http",
								Host:   "test-domain",
							},
						},
					},
					Status: duckv1.Status{
						Conditions: []apis.Condition{{
							Type:   InMemoryChannelConditionAddressable,
							Status: corev1.ConditionTrue,
						}, {
							// Note: Ready is here because when the condition
							// is marked True, duck automatically sets Ready to
							// Unknown because of missing ChannelConditionBackingChannelReady.
							Type:   InMemoryChannelConditionReady,
							Status: corev1.ConditionUnknown,
						}},
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			cs := &InMemoryChannelStatus{}
			cs.SetAddress(&duckv1.Addressable{URL: tc.url})
			if diff := cmp.Diff(tc.want, cs, ignoreAllButTypeAndStatus); diff != "" {
				t.Error("unexpected conditions (-want, +got) =", diff)
			}
		})
	}
}

func ReadyBrokerStatusWithoutDLS() *InMemoryChannelStatus {
	imcs := &InMemoryChannelStatus{}
	imcs.MarkChannelServiceTrue()
	imcs.MarkEventPoliciesTrue()
	imcs.MarkDeadLetterSinkNotConfigured()
	imcs.MarkEndpointsTrue()
	imcs.SetAddress(&duckv1.Addressable{URL: apis.HTTP("example.com")})
	imcs.MarkServiceTrue()
	return imcs
}
