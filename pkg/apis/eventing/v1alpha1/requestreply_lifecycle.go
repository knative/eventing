/*
Copyright 2024 The Knative Authors

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
	"knative.dev/pkg/apis"
	v1 "knative.dev/pkg/apis/duck/v1"
)

var requestReplyCondSet = apis.NewLivingConditionSet(RequestReplyConditionTriggers, RequestReplyConditionAddressable, RequestReplyConditionEventPoliciesReady, RequestReplyConditionBrokerReady)

const (
	RequestReplyConditionReady                                 = apis.ConditionReady
	RequestReplyConditionTriggers           apis.ConditionType = "TriggersReady"
	RequestReplyConditionAddressable        apis.ConditionType = "Addressable"
	RequestReplyConditionEventPoliciesReady apis.ConditionType = "EventPoliciesReady"
	RequestReplyConditionBrokerReady        apis.ConditionType = "BrokerReady"
)

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*RequestReply) GetConditionSet() apis.ConditionSet {
	return requestReplyCondSet
}

func (*RequestReplyStatus) GetConditionSet() apis.ConditionSet {
	return requestReplyCondSet
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (rr *RequestReplyStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return requestReplyCondSet.Manage(rr).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (rr *RequestReplyStatus) IsReady() bool {
	return rr.GetTopLevelCondition().IsTrue()
}

// GetTopLevelCondition returns the top level Condition.
func (rr *RequestReplyStatus) GetTopLevelCondition() *apis.Condition {
	return requestReplyCondSet.Manage(rr).GetTopLevelCondition()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (rr *RequestReplyStatus) InitializeConditions() {
	requestReplyCondSet.Manage(rr).InitializeConditions()
}

func (rr *RequestReplyStatus) SetAddress(address *v1.Addressable) {
	rr.AddressStatus = v1.AddressStatus{
		Address: address,
	}

	if address != nil && address.URL != nil {
		rr.GetConditionSet().Manage(rr).MarkTrue(RequestReplyConditionAddressable)
		rr.AddressStatus.Address.Name = &address.URL.Scheme
	} else {
		rr.GetConditionSet().Manage(rr).MarkFalse(RequestReplyConditionAddressable, "nil URL", "URL is nil")
	}
}

func (rr *RequestReplyStatus) MarkTriggersReady() {
	rr.GetConditionSet().Manage(rr).MarkTrue(RequestReplyConditionTriggers)
}

func (rr *RequestReplyStatus) MarkTriggersNotReadyWithReason(reason, messageFormat string, messageA ...interface{}) {
	rr.GetConditionSet().Manage(rr).MarkUnknown(RequestReplyConditionTriggers, reason, messageFormat, messageA...)
}

func (rr *RequestReplyStatus) MarkEventPoliciesTrue() {
	rr.GetConditionSet().Manage(rr).MarkTrue(RequestReplyConditionEventPoliciesReady)
}

func (rr *RequestReplyStatus) MarkEventPoliciesTrueWithReason(reason, messageFormat string, messageA ...interface{}) {
	rr.GetConditionSet().Manage(rr).MarkTrueWithReason(RequestReplyConditionEventPoliciesReady, reason, messageFormat, messageA...)
}

func (rr *RequestReplyStatus) MarkEventPoliciesFailed(reason, messageFormat string, messageA ...interface{}) {
	rr.GetConditionSet().Manage(rr).MarkFalse(RequestReplyConditionEventPoliciesReady, reason, messageFormat, messageA...)
}

func (rr *RequestReplyStatus) MarkEventPoliciesUnknown(reason, messageFormat string, messageA ...interface{}) {
	rr.GetConditionSet().Manage(rr).MarkUnknown(RequestReplyConditionEventPoliciesReady, reason, messageFormat, messageA...)
}

func (rr *RequestReplyStatus) MarkBrokerReady() {
	rr.GetConditionSet().Manage(rr).MarkTrue(RequestReplyConditionBrokerReady)
}

func (rr *RequestReplyStatus) MarkBrokerNotReady(reason, messageFormat string, messageA ...interface{}) {
	rr.GetConditionSet().Manage(rr).MarkFalse(RequestReplyConditionBrokerReady, reason, messageFormat, messageA...)
}

func (rr *RequestReplyStatus) MarkBrokerUnknown(reason, messageFormat string, messageA ...interface{}) {
	rr.GetConditionSet().Manage(rr).MarkUnknown(RequestReplyConditionBrokerReady, reason, messageFormat, messageA...)
}
