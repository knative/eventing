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

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
)

var brokerCondSet = duckv1alpha1.NewLivingConditionSet(
	BrokerConditionIngress,
	BrokerConditionTriggerChannel,
	BrokerConditionIngressChannel,
	BrokerConditionFilter,
	BrokerConditionAddressable,
	BrokerConditionIngressSubscription,
)

const (
	BrokerConditionReady                              = duckv1alpha1.ConditionReady
	BrokerConditionIngress duckv1alpha1.ConditionType = "IngressReady"

	BrokerConditionTriggerChannel duckv1alpha1.ConditionType = "TriggerChannelReady"

	BrokerConditionIngressChannel duckv1alpha1.ConditionType = "IngressChannelReady"

	BrokerConditionIngressSubscription duckv1alpha1.ConditionType = "IngressSubscriptionReady"

	BrokerConditionFilter duckv1alpha1.ConditionType = "FilterReady"

	BrokerConditionAddressable duckv1alpha1.ConditionType = "Addressable"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (bs *BrokerStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
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

func (bs *BrokerStatus) PropagateIngressDeploymentAvailability(d *v1.Deployment) {
	if deploymentIsAvailable(&d.Status) {
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

func (bs *BrokerStatus) PropagateTriggerChannelReadiness(cs *ChannelStatus) {
	if cs.IsReady() {
		brokerCondSet.Manage(bs).MarkTrue(BrokerConditionTriggerChannel)
	} else {
		msg := "nil"
		if cc := chanCondSet.Manage(cs).GetCondition(ChannelConditionReady); cc != nil {
			msg = cc.Message
		}
		bs.MarkTriggerChannelFailed("ChannelNotReady", "trigger Channel is not ready: %s", msg)
	}
}

func (bs *BrokerStatus) MarkIngressChannelFailed(reason, format string, args ...interface{}) {
	brokerCondSet.Manage(bs).MarkFalse(BrokerConditionIngressChannel, reason, format, args...)
}

func (bs *BrokerStatus) PropagateIngressChannelReadiness(cs *ChannelStatus) {
	if cs.IsReady() {
		brokerCondSet.Manage(bs).MarkTrue(BrokerConditionIngressChannel)
	} else {
		msg := "nil"
		if cc := chanCondSet.Manage(cs).GetCondition(ChannelConditionReady); cc != nil {
			msg = cc.Message
		}
		bs.MarkIngressChannelFailed("ChannelNotReady", "ingress Channel is not ready: %s", msg)
	}
}

func (bs *BrokerStatus) MarkIngressSubscriptionFailed(reason, format string, args ...interface{}) {
	brokerCondSet.Manage(bs).MarkFalse(BrokerConditionIngressSubscription, reason, format, args...)
}

func (bs *BrokerStatus) PropagateIngressSubscriptionReadiness(ss *SubscriptionStatus) {
	if ss.IsReady() {
		brokerCondSet.Manage(bs).MarkTrue(BrokerConditionIngressSubscription)
	} else {
		msg := "nil"
		if sc := subCondSet.Manage(ss).GetCondition(SubscriptionConditionReady); sc != nil {
			msg = sc.Message
		}
		bs.MarkIngressSubscriptionFailed("SubscriptionNotReady", "ingress Subscription is not ready: %s", msg)
	}
}

func (bs *BrokerStatus) MarkFilterFailed(reason, format string, args ...interface{}) {
	brokerCondSet.Manage(bs).MarkFalse(BrokerConditionFilter, reason, format, args...)
}

func (bs *BrokerStatus) PropagateFilterDeploymentAvailability(d *v1.Deployment) {
	if deploymentIsAvailable(&d.Status) {
		brokerCondSet.Manage(bs).MarkTrue(BrokerConditionFilter)
	} else {
		// I don't know how to propagate the status well, so just give the name of the Deployment
		// for now.
		bs.MarkFilterFailed("DeploymentUnavailable", "The Deployment '%s' is unavailable.", d.Name)
	}
}

func deploymentIsAvailable(d *v1.DeploymentStatus) bool {
	// Check if the Deployment is available.
	for _, cond := range d.Conditions {
		if cond.Type == v1.DeploymentAvailable {
			return cond.Status == "True"
		}
	}
	// Unable to find the Available condition, fail open.
	return true
}

// SetAddress makes this Broker addressable by setting the hostname. It also
// sets the BrokerConditionAddressable to true.
func (bs *BrokerStatus) SetAddress(hostname string) {
	bs.Address.Hostname = hostname
	if hostname != "" {
		brokerCondSet.Manage(bs).MarkTrue(BrokerConditionAddressable)
	} else {
		brokerCondSet.Manage(bs).MarkFalse(BrokerConditionAddressable, "emptyHostname", "hostname is the empty string")
	}
}
