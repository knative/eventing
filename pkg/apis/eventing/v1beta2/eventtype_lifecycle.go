/*
Copyright 2021 The Knative Authors

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

package v1beta2

import (
	"knative.dev/pkg/apis"
)

var eventTypeCondSet = apis.NewLivingConditionSet(EventTypeConditionReferenceExists)

const (
	EventTypeConditionReady                              = apis.ConditionReady
	EventTypeConditionReferenceExists apis.ConditionType = "ReferenceExists"
)

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*EventType) GetConditionSet() apis.ConditionSet {
	return eventTypeCondSet
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (et *EventTypeStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return eventTypeCondSet.Manage(et).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (et *EventTypeStatus) IsReady() bool {
	return eventTypeCondSet.Manage(et).IsHappy()
}

// GetTopLevelCondition returns the top level Condition.
func (et *EventTypeStatus) GetTopLevelCondition() *apis.Condition {
	return eventTypeCondSet.Manage(et).GetTopLevelCondition()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (et *EventTypeStatus) InitializeConditions() {
	eventTypeCondSet.Manage(et).InitializeConditions()
}

func (et *EventTypeStatus) MarkReferenceExists() {
	eventTypeCondSet.Manage(et).MarkTrue(EventTypeConditionReferenceExists)
}

func (et *EventTypeStatus) MarkReferenceDoesNotExist() {
	eventTypeCondSet.Manage(et).MarkFalse(EventTypeConditionReferenceExists, "ResourceDoesNotExist", "Resource in spec.reference does not exist")
}

func (et *EventTypeStatus) MarkReferenceExistsUnknown(reason, messageFormat string, messageA ...interface{}) {
	eventTypeCondSet.Manage(et).MarkUnknown(EventTypeConditionReferenceExists, reason, messageFormat, messageA...)
}
