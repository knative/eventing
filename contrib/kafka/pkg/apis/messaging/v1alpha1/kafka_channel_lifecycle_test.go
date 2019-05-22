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
	"github.com/google/go-cmp/cmp/cmpopts"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var condReady = duckv1alpha1.Condition{
	Type:   KafkaChannelConditionReady,
	Status: corev1.ConditionTrue,
}

var condDispatcherReady = duckv1alpha1.Condition{
	Type:   KafkaChannelConditionDispatcherReady,
	Status: corev1.ConditionTrue,
}

var condDispatcherNotReady = duckv1alpha1.Condition{
	Type:   KafkaChannelConditionDispatcherReady,
	Status: corev1.ConditionFalse,
}

var condDispatcherServiceReady = duckv1alpha1.Condition{
	Type:   KafkaChannelConditionServiceReady,
	Status: corev1.ConditionTrue,
}

var condDispatcherEndpointsReady = duckv1alpha1.Condition{
	Type:   KafkaChannelConditionEndpointsReady,
	Status: corev1.ConditionTrue,
}

var condTopicReady = duckv1alpha1.Condition{
	Type:   KafkaChannelConditionTopicReady,
	Status: corev1.ConditionTrue,
}

var condDispatcherAddressable = duckv1alpha1.Condition{
	Type:   KafkaChannelConditionAddressable,
	Status: corev1.ConditionTrue,
}

var deploymentConditionReady = appsv1.DeploymentCondition{
	Type:   appsv1.DeploymentAvailable,
	Status: corev1.ConditionTrue,
}

var deploymentConditionNotReady = appsv1.DeploymentCondition{
	Type:   appsv1.DeploymentAvailable,
	Status: corev1.ConditionFalse,
}

var deploymentStatusReady = &appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{deploymentConditionReady}}
var deploymentStatusNotReady = &appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{deploymentConditionNotReady}}

var ignoreAllButTypeAndStatus = cmpopts.IgnoreFields(
	duckv1alpha1.Condition{},
	"LastTransitionTime", "Message", "Reason", "Severity")

var ignoreLastTransitionTime = cmpopts.IgnoreFields(duckv1alpha1.Condition{}, "LastTransitionTime")

func TestChannelGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		cs        *KafkaChannelStatus
		condQuery duckv1alpha1.ConditionType
		want      *duckv1alpha1.Condition
	}{{
		name: "single condition",
		cs: &KafkaChannelStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{
					condReady,
				},
			},
		},
		condQuery: duckv1alpha1.ConditionReady,
		want:      &condReady,
	}, {
		name: "unknown condition",
		cs: &KafkaChannelStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{
					condReady,
					condDispatcherNotReady,
				},
			},
		},
		condQuery: duckv1alpha1.ConditionType("foo"),
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
		cs   *KafkaChannelStatus
		want *KafkaChannelStatus
	}{{
		name: "empty",
		cs:   &KafkaChannelStatus{},
		want: &KafkaChannelStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
					Type:   KafkaChannelConditionAddressable,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   KafkaChannelConditionChannelServiceReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   KafkaChannelConditionDispatcherReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   KafkaChannelConditionEndpointsReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   KafkaChannelConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   KafkaChannelConditionServiceReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   KafkaChannelConditionTopicReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one false",
		cs: &KafkaChannelStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
					Type:   KafkaChannelConditionDispatcherReady,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &KafkaChannelStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
					Type:   KafkaChannelConditionAddressable,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   KafkaChannelConditionChannelServiceReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   KafkaChannelConditionDispatcherReady,
					Status: corev1.ConditionFalse,
				}, {
					Type:   KafkaChannelConditionEndpointsReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   KafkaChannelConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   KafkaChannelConditionServiceReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   KafkaChannelConditionTopicReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one true",
		cs: &KafkaChannelStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
					Type:   KafkaChannelConditionDispatcherReady,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		want: &KafkaChannelStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
					Type:   KafkaChannelConditionAddressable,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   KafkaChannelConditionChannelServiceReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   KafkaChannelConditionDispatcherReady,
					Status: corev1.ConditionTrue,
				}, {
					Type:   KafkaChannelConditionEndpointsReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   KafkaChannelConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   KafkaChannelConditionServiceReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   KafkaChannelConditionTopicReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.cs.InitializeConditions()
			if diff := cmp.Diff(test.want, test.cs, ignoreAllButTypeAndStatus); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}

func TestChannelIsReady(t *testing.T) {
	tests := []struct {
		name                    string
		markServiceReady        bool
		markChannelServiceReady bool
		setAddress              bool
		markEndpointsReady      bool
		markTopicReady          bool
		wantReady               bool
		dispatcherStatus        *appsv1.DeploymentStatus
	}{{
		name:                    "all happy",
		markServiceReady:        true,
		markChannelServiceReady: true,
		markEndpointsReady:      true,
		dispatcherStatus:        deploymentStatusReady,
		setAddress:              true,
		markTopicReady:          true,
		wantReady:               true,
	}, {
		name:                    "service not ready",
		markServiceReady:        false,
		markChannelServiceReady: false,
		markEndpointsReady:      true,
		dispatcherStatus:        deploymentStatusReady,
		setAddress:              true,
		markTopicReady:          true,
		wantReady:               false,
	}, {
		name:                    "endpoints not ready",
		markServiceReady:        true,
		markChannelServiceReady: false,
		markEndpointsReady:      false,
		dispatcherStatus:        deploymentStatusReady,
		setAddress:              true,
		markTopicReady:          true,
		wantReady:               false,
	}, {
		name:                    "deployment not ready",
		markServiceReady:        true,
		markEndpointsReady:      true,
		markChannelServiceReady: false,
		dispatcherStatus:        deploymentStatusNotReady,
		setAddress:              true,
		markTopicReady:          true,
		wantReady:               false,
	}, {
		name:                    "address not set",
		markServiceReady:        true,
		markChannelServiceReady: false,
		markEndpointsReady:      true,
		dispatcherStatus:        deploymentStatusReady,
		setAddress:              false,
		markTopicReady:          true,
		wantReady:               false,
	}, {
		name:                    "channel service not ready",
		markServiceReady:        true,
		markChannelServiceReady: false,
		markEndpointsReady:      true,
		dispatcherStatus:        deploymentStatusReady,
		setAddress:              true,
		markTopicReady:          true,
		wantReady:               false,
	}, {
		name:                    "topic not ready",
		markServiceReady:        true,
		markChannelServiceReady: true,
		markEndpointsReady:      true,
		dispatcherStatus:        deploymentStatusReady,
		setAddress:              true,
		markTopicReady:          false,
		wantReady:               false,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cs := &KafkaChannelStatus{}
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
			if test.setAddress {
				cs.SetAddress("foo.bar")
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
			if test.markTopicReady {
				cs.MarkTopicTrue()
			} else {
				cs.MarkTopicFailed("NotReadyTopic", "testing")
			}
			got := cs.IsReady()
			if test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}
		})
	}
}

func TestKafkaChannelStatus_SetAddressable(t *testing.T) {
	testCases := map[string]struct {
		domainInternal string
		want           *KafkaChannelStatus
	}{
		"empty string": {
			want: &KafkaChannelStatus{
				Status: duckv1alpha1.Status{
					Conditions: []duckv1alpha1.Condition{
						{
							Type:   KafkaChannelConditionAddressable,
							Status: corev1.ConditionFalse,
						},
						// Note that Ready is here because when the condition is marked False, duck
						// automatically sets Ready to false.
						{
							Type:   KafkaChannelConditionReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
		},
		"has domain": {
			domainInternal: "test-domain",
			want: &KafkaChannelStatus{
				Address: duckv1alpha1.Addressable{
					Hostname: "test-domain",
				},
				Status: duckv1alpha1.Status{
					Conditions: []duckv1alpha1.Condition{
						{
							Type:   KafkaChannelConditionAddressable,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			cs := &KafkaChannelStatus{}
			cs.SetAddress(tc.domainInternal)
			if diff := cmp.Diff(tc.want, cs, ignoreAllButTypeAndStatus); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}
