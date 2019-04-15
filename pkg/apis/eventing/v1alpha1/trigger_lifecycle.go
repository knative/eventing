/*
 * Copyright 2019 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1alpha1

import duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"

var triggerCondSet = duckv1alpha1.NewLivingConditionSet(TriggerConditionBrokerExists, TriggerConditionSubscribed)

const (
	// TriggerConditionReady has status True when all subconditions below have been set to True.
	TriggerConditionReady                                   = duckv1alpha1.ConditionReady
	TriggerConditionBrokerExists duckv1alpha1.ConditionType = "BrokerExists"

	TriggerConditionSubscribed duckv1alpha1.ConditionType = "Subscribed"

	// TriggerAnyFilter Constant to represent that we should allow anything.
	TriggerAnyFilter = ""
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (ts *TriggerStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return triggerCondSet.Manage(ts).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (ts *TriggerStatus) IsReady() bool {
	return triggerCondSet.Manage(ts).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ts *TriggerStatus) InitializeConditions() {
	triggerCondSet.Manage(ts).InitializeConditions()
}

func (ts *TriggerStatus) MarkBrokerExists() {
	triggerCondSet.Manage(ts).MarkTrue(TriggerConditionBrokerExists)
}

func (ts *TriggerStatus) MarkBrokerDoesNotExist() {
	triggerCondSet.Manage(ts).MarkFalse(TriggerConditionBrokerExists, "doesNotExist", "Broker does not exist")
}

func (ts *TriggerStatus) MarkSubscribed() {
	triggerCondSet.Manage(ts).MarkTrue(TriggerConditionSubscribed)
}

func (ts *TriggerStatus) MarkNotSubscribed(reason, messageFormat string, messageA ...interface{}) {
	triggerCondSet.Manage(ts).MarkFalse(TriggerConditionSubscribed, reason, messageFormat, messageA...)
}
