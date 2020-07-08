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
	corev1 "k8s.io/api/core/v1"

	"knative.dev/eventing/pkg/apis/duck"
	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/eventing"

	"knative.dev/pkg/apis"
)

var brokerCondSet = apis.NewLivingConditionSet(
	BrokerConditionIngress,
	BrokerConditionTriggerChannel,
	BrokerConditionFilter,
	BrokerConditionAddressable,
)

var customConditionSet = map[string]apis.ConditionSet{
	"":                                 brokerCondSet,
	eventing.MTChannelBrokerClassValue: brokerCondSet,
}

// RegisterAlternateBrokerConditionSet register a apis.ConditionSet for the given broker class.
//
// Calls to this function need to be synchronized by the caller (not thread-safe).
func RegisterAlternateBrokerConditionSet(brokerClass string, conditionSet apis.ConditionSet) {
	customConditionSet[brokerClass] = conditionSet
}

const (
	BrokerConditionReady                             = apis.ConditionReady
	BrokerConditionIngress        apis.ConditionType = "IngressReady"
	BrokerConditionTriggerChannel apis.ConditionType = "TriggerChannelReady"
	BrokerConditionFilter         apis.ConditionType = "FilterReady"
	BrokerConditionAddressable    apis.ConditionType = "Addressable"
)

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (b *Broker) GetConditionSet() apis.ConditionSet {

	annotations := b.GetAnnotations()
	if annotations != nil {
		if brokerClass, ok := annotations[eventing.BrokerClassKey]; ok && brokerClass != eventing.MTChannelBrokerClassValue {

			// Set broker class as annotation of the status, so that we can use it.
			if b.Status.Annotations == nil {
				b.Status.Annotations = map[string]string{eventing.BrokerClassKey: brokerClass}
			} else {
				b.Status.Annotations[eventing.BrokerClassKey] = brokerClass
			}

			return customConditionSet[brokerClass]
		}
	}

	return brokerCondSet
}

// GetConditionSet retrieves the condition set for this resource.
func (bs *BrokerStatus) GetConditionSet() apis.ConditionSet {
	return customConditionSet[bs.Annotations[eventing.BrokerClassKey]]
}

// GetTopLevelCondition returns the top level Condition.
func (bs *BrokerStatus) GetTopLevelCondition() *apis.Condition {
	return bs.GetConditionSet().Manage(bs).GetTopLevelCondition()
}

// SetAddress makes this Broker addressable by setting the URI. It also
// sets the BrokerConditionAddressable to true.
func (bs *BrokerStatus) SetAddress(url *apis.URL) {
	bs.Address.URL = url
	if url != nil {
		bs.GetConditionSet().Manage(bs).MarkTrue(BrokerConditionAddressable)
	} else {
		bs.GetConditionSet().Manage(bs).MarkFalse(BrokerConditionAddressable, "nil URL", "URL is nil")
	}
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (bs *BrokerStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return bs.GetConditionSet().Manage(bs).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (bs *BrokerStatus) IsReady() bool {
	return bs.GetConditionSet().Manage(bs).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (bs *BrokerStatus) InitializeConditions() {
	bs.GetConditionSet().Manage(bs).InitializeConditions()
}

func (bs *BrokerStatus) MarkIngressFailed(reason, format string, args ...interface{}) {
	bs.GetConditionSet().Manage(bs).MarkFalse(BrokerConditionIngress, reason, format, args...)
}

func (bs *BrokerStatus) PropagateIngressAvailability(ep *corev1.Endpoints) {
	if duck.EndpointsAreAvailable(ep) {
		bs.GetConditionSet().Manage(bs).MarkTrue(BrokerConditionIngress)
	} else {
		bs.MarkIngressFailed("EndpointsUnavailable", "Endpoints %q are unavailable.", ep.Name)
	}
}

func (bs *BrokerStatus) MarkTriggerChannelFailed(reason, format string, args ...interface{}) {
	bs.GetConditionSet().Manage(bs).MarkFalse(BrokerConditionTriggerChannel, reason, format, args...)
}

func (bs *BrokerStatus) PropagateTriggerChannelReadiness(cs *duckv1.ChannelableStatus) {
	// TODO: Once you can get a Ready status from Channelable in a generic way, use it here...
	address := cs.AddressStatus.Address
	if address != nil {
		bs.GetConditionSet().Manage(bs).MarkTrue(BrokerConditionTriggerChannel)
	} else {
		bs.MarkTriggerChannelFailed("ChannelNotReady", "trigger Channel is not ready: not addressable")
	}
}

func (bs *BrokerStatus) MarkFilterFailed(reason, format string, args ...interface{}) {
	bs.GetConditionSet().Manage(bs).MarkFalse(BrokerConditionFilter, reason, format, args...)
}

func (bs *BrokerStatus) PropagateFilterAvailability(ep *corev1.Endpoints) {
	if duck.EndpointsAreAvailable(ep) {
		bs.GetConditionSet().Manage(bs).MarkTrue(BrokerConditionFilter)
	} else {
		bs.MarkFilterFailed("EndpointsUnavailable", "Endpoints %q are unavailable.", ep.Name)
	}
}
