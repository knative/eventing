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
	v1 "k8s.io/api/apps/v1"
	"knative.dev/pkg/apis"
	pkgduckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

type testHelper struct{}

// TestHelper contains helpers for unit tests.
var TestHelper = testHelper{}

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

func (t testHelper) NotReadyChannelStatus() *duckv1alpha1.ChannelableStatus {
	return &duckv1alpha1.ChannelableStatus{}
}

func (testHelper) ReadySubscriptionStatus() *messagingv1alpha1.SubscriptionStatus {
	ss := &messagingv1alpha1.SubscriptionStatus{}
	ss.MarkChannelReady()
	ss.MarkReferencesResolved()
	ss.MarkAddedToChannel()
	return ss
}

func (testHelper) FalseSubscriptionStatus() *messagingv1alpha1.SubscriptionStatus {
	ss := &messagingv1alpha1.SubscriptionStatus{}
	ss.MarkReferencesResolved()
	ss.MarkChannelFailed("testInducedError", "test induced %s", "error")
	return ss
}

func (t testHelper) ReadyBrokerStatus() *BrokerStatus {
	bs := &BrokerStatus{}
	bs.PropagateIngressDeploymentAvailability(t.AvailableDeployment())
	bs.PropagateTriggerChannelReadiness(t.ReadyChannelStatus())
	bs.PropagateFilterDeploymentAvailability(t.AvailableDeployment())
	bs.SetAddress(&apis.URL{Scheme: "http", Host: "foo"})
	return bs
}

func (t testHelper) UnknownBrokerStatus() *BrokerStatus {
	bs := &BrokerStatus{}
	bs.InitializeConditions()
	return bs
}

func (t testHelper) FalseBrokerStatus() *BrokerStatus {
	bs := &BrokerStatus{}
	bs.MarkIngressFailed("DeploymentUnavailable", "The Deployment is unavailable.")
	bs.MarkTriggerChannelFailed("ChannelNotReady", "trigger Channel is not ready: not addressalbe")
	bs.MarkFilterFailed("DeploymentUnavailable", "The Deployment is unavailable.")
	bs.SetAddress(nil)
	return bs
}

func (t testHelper) ReadyTriggerStatus() *TriggerStatus {
	ts := &TriggerStatus{}
	ts.InitializeConditions()
	ts.SubscriberURI = &apis.URL{Scheme: "http", Host: "foo"}
	ts.PropagateBrokerStatus(t.ReadyBrokerStatus())
	ts.PropagateSubscriptionStatus(t.ReadySubscriptionStatus())
	return ts
}

func (testHelper) UnavailableDeployment() *v1.Deployment {
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

func (t testHelper) AvailableDeployment() *v1.Deployment {
	d := t.UnavailableDeployment()
	d.Name = "available"
	d.Status.Conditions = []v1.DeploymentCondition{
		{
			Type:   v1.DeploymentAvailable,
			Status: "True",
		},
	}
	return d
}
