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

	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/pkg/apis"
)

type testHelper struct{}

// TestHelper contains helpers for unit tests.
var TestHelper = testHelper{}

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
	bs.SetAddress(apis.HTTP("example.com"))
	return bs
}

func (t testHelper) UnknownBrokerStatus() *BrokerStatus {
	bs := &BrokerStatus{}
	return bs
}

func (t testHelper) FalseBrokerStatus() *BrokerStatus {
	bs := &BrokerStatus{}
	bs.SetAddress(nil)
	return bs
}
