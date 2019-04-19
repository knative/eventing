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
)

type testHelper struct{}

// TestHelper contains helpers for unit tests.
var TestHelper = testHelper{}

func (testHelper) ReadyChannelStatus() *ChannelStatus {
	cs := &ChannelStatus{}
	cs.MarkProvisionerInstalled()
	cs.MarkProvisioned()
	cs.SetAddress("foo")
	return cs
}

func (t testHelper) NotReadyChannelStatus() *ChannelStatus {
	cs := t.ReadyChannelStatus()
	cs.MarkNotProvisioned("foo", "bar")
	return cs
}

func (testHelper) ReadySubscriptionStatus() *SubscriptionStatus {
	ss := &SubscriptionStatus{}
	ss.MarkChannelReady()
	ss.MarkReferencesResolved()
	return ss
}

func (testHelper) NotReadySubscriptionStatus() *SubscriptionStatus {
	ss := &SubscriptionStatus{}
	ss.MarkReferencesResolved()
	return ss
}

func (t testHelper) ReadyBrokerStatus() *BrokerStatus {
	bs := &BrokerStatus{}
	bs.PropagateIngressDeploymentAvailability(t.AvailableDeployment())
	bs.PropagateIngressChannelReadiness(t.ReadyChannelStatus())
	bs.PropagateTriggerChannelReadiness(t.ReadyChannelStatus())
	bs.PropagateIngressSubscriptionReadiness(t.ReadySubscriptionStatus())
	bs.PropagateFilterDeploymentAvailability(t.AvailableDeployment())
	bs.SetAddress("foo")
	return bs
}

func (t testHelper) ReadyTriggerStatus() *TriggerStatus {
	ts := &TriggerStatus{}
	ts.InitializeConditions()
	ts.SubscriberURI = "http://foo/"
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
