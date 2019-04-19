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

// testhelper contains helpers for unit tests.
package testhelper

import (
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	v1 "k8s.io/api/apps/v1"
)

func ReadyChannelStatus() *v1alpha1.ChannelStatus {
	cs := &v1alpha1.ChannelStatus{}
	cs.MarkProvisionerInstalled()
	cs.MarkProvisioned()
	cs.SetAddress("foo")
	return cs
}

func NotReadyChannelStatus() *v1alpha1.ChannelStatus {
	cs := ReadyChannelStatus()
	cs.MarkNotProvisioned("foo", "bar")
	return cs
}

func ReadySubscriptionStatus() *v1alpha1.SubscriptionStatus {
	ss := &v1alpha1.SubscriptionStatus{}
	ss.MarkChannelReady()
	ss.MarkReferencesResolved()
	return ss
}

func NotReadySubscriptionStatus() *v1alpha1.SubscriptionStatus {
	ss := &v1alpha1.SubscriptionStatus{}
	ss.MarkReferencesResolved()
	return ss
}

func ReadyBrokerStatus() *v1alpha1.BrokerStatus {
	bs := &v1alpha1.BrokerStatus{}
	bs.PropagateIngressDeploymentAvailability(AvailableDeployment())
	bs.PropagateIngressChannelReadiness(ReadyChannelStatus())
	bs.PropagateTriggerChannelReadiness(ReadyChannelStatus())
	bs.PropagateIngressSubscriptionReadiness(ReadySubscriptionStatus())
	bs.PropagateFilterDeploymentAvailability(AvailableDeployment())
	bs.SetAddress("foo")
	return bs
}

func ReadyTriggerStatus() *v1alpha1.TriggerStatus {
	ts := &v1alpha1.TriggerStatus{}
	ts.InitializeConditions()
	ts.SubscriberURI = "http://foo/"
	ts.PropagateBrokerStatus(ReadyBrokerStatus())
	ts.PropagateSubscriptionStatus(ReadySubscriptionStatus())
	return ts
}

func UnavailableDeployment() *v1.Deployment {
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

func AvailableDeployment() *v1.Deployment {
	d := UnavailableDeployment()
	d.Name = "available"
	d.Status.Conditions = []v1.DeploymentCondition{
		{
			Type:   v1.DeploymentAvailable,
			Status: "True",
		},
	}
	return d
}
