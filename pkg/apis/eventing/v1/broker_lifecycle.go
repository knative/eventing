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
	"sync"

	"knative.dev/pkg/apis"
	v1 "knative.dev/pkg/apis/duck/v1"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1"
)

const (
	BrokerConditionReady                                     = apis.ConditionReady
	BrokerConditionIngress                apis.ConditionType = "IngressReady"
	BrokerConditionTriggerChannel         apis.ConditionType = "TriggerChannelReady"
	BrokerConditionFilter                 apis.ConditionType = "FilterReady"
	BrokerConditionAddressable            apis.ConditionType = "Addressable"
	BrokerConditionDeadLetterSinkResolved apis.ConditionType = "DeadLetterSinkResolved"
	BrokerConditionEventPoliciesReady     apis.ConditionType = "EventPoliciesReady"
)

var brokerCondSet = apis.NewLivingConditionSet(
	BrokerConditionIngress,
	BrokerConditionTriggerChannel,
	BrokerConditionFilter,
	BrokerConditionAddressable,
	BrokerConditionDeadLetterSinkResolved,
	BrokerConditionEventPoliciesReady,
)
var brokerCondSetLock = sync.RWMutex{}

// RegisterAlternateBrokerConditionSet register a apis.ConditionSet for the given broker class.
func RegisterAlternateBrokerConditionSet(conditionSet apis.ConditionSet) {
	brokerCondSetLock.Lock()
	defer brokerCondSetLock.Unlock()

	brokerCondSet = conditionSet
}

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (b *Broker) GetConditionSet() apis.ConditionSet {
	brokerCondSetLock.RLock()
	defer brokerCondSetLock.RUnlock()

	return brokerCondSet
}

// GetConditionSet retrieves the condition set for this resource.
func (bs *BrokerStatus) GetConditionSet() apis.ConditionSet {
	brokerCondSetLock.RLock()
	defer brokerCondSetLock.RUnlock()

	return brokerCondSet
}

// GetTopLevelCondition returns the top level Condition.
func (bs *BrokerStatus) GetTopLevelCondition() *apis.Condition {
	return bs.GetConditionSet().Manage(bs).GetTopLevelCondition()
}

// SetAddress makes this Broker addressable by setting the URI. It also
// sets the BrokerConditionAddressable to true.
func (bs *BrokerStatus) SetAddress(address *v1.Addressable) {
	bs.AddressStatus = v1.AddressStatus{
		Address: address,
	}

	if address != nil && address.URL != nil {
		bs.GetConditionSet().Manage(bs).MarkTrue(BrokerConditionAddressable)
		bs.AddressStatus.Address.Name = &address.URL.Scheme
	} else {
		bs.GetConditionSet().Manage(bs).MarkFalse(BrokerConditionAddressable, "nil URL", "URL is nil")
	}
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (bs *BrokerStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return bs.GetConditionSet().Manage(bs).GetCondition(t)
}

// IsReady returns true if the resource is ready overall and the latest spec has been observed.
func (b *Broker) IsReady() bool {
	bs := b.Status
	return bs.ObservedGeneration == b.Generation &&
		b.GetConditionSet().Manage(&bs).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (bs *BrokerStatus) InitializeConditions() {
	bs.GetConditionSet().Manage(bs).InitializeConditions()
}

func (bs *BrokerStatus) MarkDeadLetterSinkResolvedSucceeded(deadLetterSink eventingduck.DeliveryStatus) {
	bs.DeliveryStatus = deadLetterSink
	bs.GetConditionSet().Manage(bs).MarkTrue(BrokerConditionDeadLetterSinkResolved)
}

func (bs *BrokerStatus) MarkDeadLetterSinkNotConfigured() {
	bs.DeliveryStatus = eventingduck.DeliveryStatus{}
	bs.GetConditionSet().Manage(bs).MarkTrueWithReason(BrokerConditionDeadLetterSinkResolved, "DeadLetterSinkNotConfigured", "No dead letter sink is configured.")
}

func (bs *BrokerStatus) MarkDeadLetterSinkResolvedFailed(reason, messageFormat string, messageA ...interface{}) {
	bs.DeliveryStatus = eventingduck.DeliveryStatus{}
	bs.GetConditionSet().Manage(bs).MarkFalse(BrokerConditionDeadLetterSinkResolved, reason, messageFormat, messageA...)
}

func (bs *BrokerStatus) MarkEventPoliciesTrue() {
	bs.GetConditionSet().Manage(bs).MarkTrue(BrokerConditionEventPoliciesReady)
}

func (bs *BrokerStatus) MarkEventPoliciesTrueWithReason(reason, messageFormat string, messageA ...interface{}) {
	bs.GetConditionSet().Manage(bs).MarkTrueWithReason(BrokerConditionEventPoliciesReady, reason, messageFormat, messageA...)
}

func (bs *BrokerStatus) MarkEventPoliciesFailed(reason, messageFormat string, messageA ...interface{}) {
	bs.GetConditionSet().Manage(bs).MarkFalse(BrokerConditionEventPoliciesReady, reason, messageFormat, messageA...)
}

func (bs *BrokerStatus) MarkEventPoliciesUnknown(reason, messageFormat string, messageA ...interface{}) {
	bs.GetConditionSet().Manage(bs).MarkUnknown(BrokerConditionEventPoliciesReady, reason, messageFormat, messageA...)
}
