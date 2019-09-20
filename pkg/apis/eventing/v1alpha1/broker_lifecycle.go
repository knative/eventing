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
	appsv1 "k8s.io/api/apps/v1"
	"knative.dev/pkg/apis"

	"knative.dev/eventing/pkg/apis/duck"
	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
)

var brokerCondSet = apis.NewLivingConditionSet(
	BrokerConditionIngress,
	BrokerConditionTriggerChannel,
	BrokerConditionIngressChannel,
	BrokerConditionFilter,
	BrokerConditionAddressable,
	BrokerConditionIngressSubscription,
)

const (
	BrokerConditionReady                      = apis.ConditionReady
	BrokerConditionIngress apis.ConditionType = "IngressReady"

	BrokerConditionTriggerChannel apis.ConditionType = "TriggerChannelReady"

	BrokerConditionIngressChannel apis.ConditionType = "IngressChannelReady"

	BrokerConditionIngressSubscription apis.ConditionType = "IngressSubscriptionReady"

	BrokerConditionFilter apis.ConditionType = "FilterReady"

	BrokerConditionAddressable apis.ConditionType = "Addressable"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (bs *BrokerStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return brokerCondSet.Manage(bs).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (bs *BrokerStatus) IsReady() bool {
	return brokerCondSet.Manage(bs).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (bs *BrokerStatus) InitializeConditions() {
	brokerCondSet.Manage(bs).InitializeConditions()
}

func (bs *BrokerStatus) MarkIngressFailed(reason, format string, args ...interface{}) {
	brokerCondSet.Manage(bs).MarkFalse(BrokerConditionIngress, reason, format, args...)
}

func (bs *BrokerStatus) PropagateIngressDeploymentAvailability(d *appsv1.Deployment) {
	if duck.DeploymentIsAvailable(&d.Status, true) {
		brokerCondSet.Manage(bs).MarkTrue(BrokerConditionIngress)
	} else {
		// I don't know how to propagate the status well, so just give the name of the Deployment
		// for now.
		bs.MarkIngressFailed("DeploymentUnavailable", "The Deployment '%s' is unavailable.", d.Name)
	}
}

func (bs *BrokerStatus) MarkTriggerChannelFailed(reason, format string, args ...interface{}) {
	brokerCondSet.Manage(bs).MarkFalse(BrokerConditionTriggerChannel, reason, format, args...)
}

func (bs *BrokerStatus) PropagateTriggerChannelReadiness(cs *duckv1alpha1.ChannelableStatus) {
	// TODO: Once you can get a Ready status from Channelable in a generic way, use it here...
	address := cs.AddressStatus.Address
	if address != nil {
		brokerCondSet.Manage(bs).MarkTrue(BrokerConditionTriggerChannel)
	} else {
		bs.MarkTriggerChannelFailed("ChannelNotReady", "trigger Channel is not ready: not addressalbe")
	}
}

func (bs *BrokerStatus) MarkIngressChannelFailed(reason, format string, args ...interface{}) {
	brokerCondSet.Manage(bs).MarkFalse(BrokerConditionIngressChannel, reason, format, args...)
}

func (bs *BrokerStatus) PropagateIngressChannelReadiness(cs *duckv1alpha1.ChannelableStatus) {
	// TODO: Once you can get a Ready status from Channelable in a generic way, use it here...
	address := cs.AddressStatus.Address
	if address != nil {
		brokerCondSet.Manage(bs).MarkTrue(BrokerConditionIngressChannel)
	} else {
		bs.MarkIngressChannelFailed("ChannelNotReady", "ingress Channel is not ready: not addressable")
	}
}

func (bs *BrokerStatus) MarkIngressSubscriptionFailed(reason, format string, args ...interface{}) {
	brokerCondSet.Manage(bs).MarkFalse(BrokerConditionIngressSubscription, reason, format, args...)
}

func (bs *BrokerStatus) MarkIngressSubscriptionNotOwned(sub *messagingv1alpha1.Subscription) {
	bs.MarkIngressSubscriptionFailed("SubscriptionNotOwned", "Subscription %q is not owned by this Broker.", sub.Name)
}

func (bs *BrokerStatus) PropagateIngressSubscriptionReadiness(ss *messagingv1alpha1.SubscriptionStatus) {
	if ss.IsReady() {
		brokerCondSet.Manage(bs).MarkTrue(BrokerConditionIngressSubscription)
	} else {
		msg := "nil"
		if sc := ss.GetCondition(messagingv1alpha1.SubscriptionConditionReady); sc != nil {
			msg = sc.Message
		}
		bs.MarkIngressSubscriptionFailed("SubscriptionNotReady", "ingress Subscription is not ready: %s", msg)
	}
}

func (bs *BrokerStatus) MarkFilterFailed(reason, format string, args ...interface{}) {
	brokerCondSet.Manage(bs).MarkFalse(BrokerConditionFilter, reason, format, args...)
}

func (bs *BrokerStatus) PropagateFilterDeploymentAvailability(d *appsv1.Deployment) {
	if duck.DeploymentIsAvailable(&d.Status, true) {
		brokerCondSet.Manage(bs).MarkTrue(BrokerConditionFilter)
	} else {
		// I don't know how to propagate the status well, so just give the name of the Deployment
		// for now.
		bs.MarkFilterFailed("DeploymentUnavailable", "The Deployment '%s' is unavailable.", d.Name)
	}
}

// SetAddress makes this Broker addressable by setting the hostname. It also
// sets the BrokerConditionAddressable to true.
func (bs *BrokerStatus) SetAddress(url *apis.URL) {
	if url != nil {
		bs.Address.Hostname = url.Host
		bs.Address.URL = url
		brokerCondSet.Manage(bs).MarkTrue(BrokerConditionAddressable)
	} else {
		bs.Address.Hostname = ""
		bs.Address.URL = nil
		brokerCondSet.Manage(bs).MarkFalse(BrokerConditionAddressable, "emptyHostname", "hostname is the empty string")
	}
}
