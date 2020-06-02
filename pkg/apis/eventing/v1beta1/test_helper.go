/*
 * Copyright 2020 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"

	duckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

type testHelper struct{}

// TestHelper contains helpers for unit tests.
var TestHelper = testHelper{}

// Moved to here from ../v1alpha1 while we migrate things...
func (testHelper) ReadySubscriptionStatusV1Alpha1() *messagingv1alpha1.SubscriptionStatus {
	ss := &messagingv1alpha1.SubscriptionStatus{}
	ss.MarkChannelReady()
	ss.MarkReferencesResolved()
	ss.MarkAddedToChannel()
	return ss
}

func (testHelper) ReadySubscriptionCondition() *apis.Condition {
	return &apis.Condition{
		Type:     apis.ConditionReady,
		Status:   corev1.ConditionTrue,
		Severity: apis.ConditionSeverityError,
	}
}

func (testHelper) FalseSubscriptionCondition() *apis.Condition {
	return &apis.Condition{
		Type:     apis.ConditionReady,
		Status:   corev1.ConditionFalse,
		Severity: apis.ConditionSeverityError,
		Message:  "test induced failure condition",
	}
}

func (testHelper) ReadySubscriptionStatus() *messagingv1beta1.SubscriptionStatus {
	ss := &messagingv1beta1.SubscriptionStatus{}
	ss.MarkChannelReady()
	ss.MarkReferencesResolved()
	ss.MarkAddedToChannel()
	return ss
}

func (t testHelper) ReadyBrokerStatus() *BrokerStatus {
	bs := &BrokerStatus{}
	bs.PropagateIngressAvailability(t.AvailableEndpoints())
	bs.PropagateTriggerChannelReadiness(t.ReadyChannelStatus())
	bs.PropagateFilterAvailability(t.AvailableEndpoints())
	bs.SetAddress(apis.HTTP("example.com"))
	return bs
}

func (testHelper) ReadyBrokerCondition() *apis.Condition {
	return &apis.Condition{
		Type:     apis.ConditionReady,
		Status:   corev1.ConditionTrue,
		Severity: apis.ConditionSeverityError,
	}
}

func (testHelper) UnknownBrokerStatus() *BrokerStatus {
	bs := &BrokerStatus{}
	return bs
}

func (testHelper) FalseBrokerStatus() *BrokerStatus {
	bs := &BrokerStatus{}
	bs.SetAddress(nil)
	return bs
}

func (testHelper) UnavailableEndpoints() *corev1.Endpoints {
	ep := &corev1.Endpoints{}
	ep.Name = "unavailable"
	ep.Subsets = []corev1.EndpointSubset{{
		NotReadyAddresses: []corev1.EndpointAddress{{
			IP: "127.0.0.1",
		}},
	}}
	return ep
}

func (testHelper) AvailableEndpoints() *corev1.Endpoints {
	ep := &corev1.Endpoints{}
	ep.Name = "available"
	ep.Subsets = []corev1.EndpointSubset{{
		Addresses: []corev1.EndpointAddress{{
			IP: "127.0.0.1",
		}},
	}}
	return ep
}

func (testHelper) ReadyChannelStatus() *duckv1beta1.ChannelableStatus {
	cs := &duckv1beta1.ChannelableStatus{
		Status: duckv1.Status{},
		AddressStatus: duckv1.AddressStatus{
			Address: &duckv1.Addressable{
				URL: &apis.URL{Scheme: "http", Host: "foo"},
			},
		},
		SubscribableStatus: duckv1beta1.SubscribableStatus{}}
	return cs
}

func (t testHelper) NotReadyChannelStatus() *duckv1beta1.ChannelableStatus {
	return &duckv1beta1.ChannelableStatus{}
}
