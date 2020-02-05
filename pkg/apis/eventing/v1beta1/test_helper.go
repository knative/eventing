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
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgduckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
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

func (testHelper) ReadyChannelStatus() *duckv1alpha1.ChannelableStatus {
	cs := &duckv1alpha1.ChannelableStatus{
		Status: duckv1.Status{},
		AddressStatus: pkgduckv1alpha1.AddressStatus{
			Address: &pkgduckv1alpha1.Addressable{
				Addressable: duckv1beta1.Addressable{
					URL: &apis.URL{Scheme: "http", Host: "foo"},
				},
				Hostname: "foo",
			},
		},
		SubscribableTypeStatus: duckv1alpha1.SubscribableTypeStatus{}}
	return cs
}

func (t testHelper) ReadyBrokerStatus() *BrokerStatus {
	bs := &BrokerStatus{}
	bs.SetAddress(apis.HTTP("example.com"))
	return bs
}

func (t testHelper) AvailableDeployment() *v1.Deployment {
	d := &v1.Deployment{}
	d.Name = "available"
	d.Status.Conditions = []v1.DeploymentCondition{
		{
			Type:   v1.DeploymentAvailable,
			Status: "True",
		},
	}
	return d
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
