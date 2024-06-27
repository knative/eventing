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
)

var eventPolicyCondSet = apis.NewLivingConditionSet(EventPolicyConditionAuthenticationEnabled, EventPolicyConditionSubjectsResolved)

const (
	EventPolicyConditionReady                                    = apis.ConditionReady
	EventPolicyConditionAuthenticationEnabled apis.ConditionType = "AuthenticationEnabled"
	EventPolicyConditionSubjectsResolved      apis.ConditionType = "SubjectsResolved"
)

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*EventPolicy) GetConditionSet() apis.ConditionSet {
	return eventPolicyCondSet
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (ep *EventPolicyStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return eventPolicyCondSet.Manage(ep).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (ep *EventPolicyStatus) IsReady() bool {
	return ep.GetTopLevelCondition().IsTrue()
}

// GetTopLevelCondition returns the top level Condition.
func (ep *EventPolicyStatus) GetTopLevelCondition() *apis.Condition {
	return eventPolicyCondSet.Manage(ep).GetTopLevelCondition()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ep *EventPolicyStatus) InitializeConditions() {
	eventPolicyCondSet.Manage(ep).InitializeConditions()
}

// MarkOIDCAuthenticationEnabled sets EventPolicyConditionAuthenticationEnabled condition to true.
func (ep *EventPolicyStatus) MarkOIDCAuthenticationEnabled() {
	eventPolicyCondSet.Manage(ep).MarkTrue(EventPolicyConditionAuthenticationEnabled)
}

// MarkOIDCAuthenticationDisabled sets EventPolicyConditionAuthenticationEnabled condition to false.
func (ep *EventPolicyStatus) MarkOIDCAuthenticationDisabled(reason, messageFormat string, messageA ...interface{}) {
	eventPolicyCondSet.Manage(ep).MarkFalse(EventPolicyConditionAuthenticationEnabled, reason, messageFormat, messageA...)
}

// MarkSubjectsResolved sets EventPolicyConditionSubjectsResolved condition to true.
func (ep *EventPolicyStatus) MarkSubjectsResolvedSucceeded() {
	eventPolicyCondSet.Manage(ep).MarkTrue(EventPolicyConditionSubjectsResolved)
}

// MarkSubjectsNotResolved sets EventPolicyConditionSubjectsResolved condition to false.
func (ep *EventPolicyStatus) MarkSubjectsResolvedFailed(reason, messageFormat string, messageA ...interface{}) {
	eventPolicyCondSet.Manage(ep).MarkFalse(EventPolicyConditionSubjectsResolved, reason, messageFormat, messageA...)
}
