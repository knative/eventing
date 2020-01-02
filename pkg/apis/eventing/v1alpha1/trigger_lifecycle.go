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

import (
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var triggerCondSet = apis.NewLivingConditionSet(TriggerConditionBroker, TriggerConditionSubscribed, TriggerConditionDependency, TriggerConditionSubscriberResolved)

const (
	// TriggerConditionReady has status True when all subconditions below have been set to True.
	TriggerConditionReady = apis.ConditionReady

	TriggerConditionBroker apis.ConditionType = "Broker"

	TriggerConditionSubscribed apis.ConditionType = "Subscribed"

	TriggerConditionDependency apis.ConditionType = "Dependency"

	TriggerConditionSubscriberResolved apis.ConditionType = "SubscriberResolved"

	// TriggerAnyFilter Constant to represent that we should allow anything.
	TriggerAnyFilter = ""
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (ts *TriggerStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return triggerCondSet.Manage(ts).GetCondition(t)
}

// GetTopLevelCondition returns the top level Condition.
func (ts *TriggerStatus) GetTopLevelCondition() *apis.Condition {
	return triggerCondSet.Manage(ts).GetTopLevelCondition()
}

// IsReady returns true if the resource is ready overall.
func (ts *TriggerStatus) IsReady() bool {
	return triggerCondSet.Manage(ts).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ts *TriggerStatus) InitializeConditions() {
	triggerCondSet.Manage(ts).InitializeConditions()
}

func (ts *TriggerStatus) PropagateBrokerStatus(bs *BrokerStatus) {
	bc := brokerCondSet.Manage(bs).GetTopLevelCondition()
	if bc == nil {
		ts.MarkBrokerUnknown("BrokerUnknown", "The condition of Broker is nil")
		return
	}
	if bc.IsTrue() {
		triggerCondSet.Manage(ts).MarkTrue(TriggerConditionBroker)
	} else {
		msg := bc.Message
		if bc.IsUnknown() {
			ts.MarkBrokerUnknown("BrokerUnknown", "The status of Broker is Unknown: %s", msg)
		} else if bc.IsFalse() {
			ts.MarkBrokerFailed("BrokerFalse", "The status of Broker is False: %s", msg)
		} else {
			ts.MarkBrokerUnknown("BrokerUnknown", "The status of Broker is invalid: %v", bc.Status)
		}
	}
}

func (ts *TriggerStatus) MarkBrokerFailed(reason, messageFormat string, messageA ...interface{}) {
	triggerCondSet.Manage(ts).MarkFalse(TriggerConditionBroker, reason, messageFormat, messageA...)
}

func (ts *TriggerStatus) MarkBrokerUnknown(reason, messageFormat string, messageA ...interface{}) {
	triggerCondSet.Manage(ts).MarkUnknown(TriggerConditionBroker, reason, messageFormat, messageA...)
}

func (ts *TriggerStatus) PropagateSubscriptionStatus(ss *messagingv1alpha1.SubscriptionStatus) {
	sc := messagingv1alpha1.SubCondSet.Manage(ss).GetTopLevelCondition()
	if sc == nil {
		ts.MarkSubscribedUnknown("SubscriptionUnknown", "The condition of Subscription is nil")
		return
	}
	if sc.IsTrue() {
		triggerCondSet.Manage(ts).MarkTrue(TriggerConditionSubscribed)
	} else {
		msg := sc.Message
		if sc.IsUnknown() {
			ts.MarkSubscribedUnknown("SubscriptionUnknown", "The status of Subscription is Unknown: %s", msg)
		} else if sc.IsFalse() {
			ts.MarkNotSubscribed("SubscriptionFalse", "The status of Subscription is False: %s", msg)
		} else {
			ts.MarkSubscribedUnknown("SubscriptionUnknown", "The status of Broker is invalid: %v", sc.Status)
		}
	}
}

func (ts *TriggerStatus) MarkNotSubscribed(reason, messageFormat string, messageA ...interface{}) {
	triggerCondSet.Manage(ts).MarkFalse(TriggerConditionSubscribed, reason, messageFormat, messageA...)
}

func (ts *TriggerStatus) MarkSubscribedUnknown(reason, messageFormat string, messageA ...interface{}) {
	triggerCondSet.Manage(ts).MarkUnknown(TriggerConditionSubscribed, reason, messageFormat, messageA...)
}

func (ts *TriggerStatus) MarkSubscriptionNotOwned(sub *messagingv1alpha1.Subscription) {
	triggerCondSet.Manage(ts).MarkFalse(TriggerConditionSubscribed, "SubscriptionNotOwned", "Subscription %q is not owned by this Trigger.", sub.Name)
}

func (ts *TriggerStatus) MarkSubscriberResolvedSucceeded() {
	triggerCondSet.Manage(ts).MarkTrue(TriggerConditionSubscriberResolved)
}

func (ts *TriggerStatus) MarkSubscriberResolvedFailed(reason, messageFormat string, messageA ...interface{}) {
	triggerCondSet.Manage(ts).MarkFalse(TriggerConditionSubscriberResolved, reason, messageFormat, messageA...)
}

func (ts *TriggerStatus) MarkSubscriberResolvedUnknown(reason, messageFormat string, messageA ...interface{}) {
	triggerCondSet.Manage(ts).MarkUnknown(TriggerConditionSubscriberResolved, reason, messageFormat, messageA...)
}

func (ts *TriggerStatus) MarkDependencySucceeded() {
	triggerCondSet.Manage(ts).MarkTrue(TriggerConditionDependency)
}

func (ts *TriggerStatus) MarkDependencyFailed(reason, messageFormat string, messageA ...interface{}) {
	triggerCondSet.Manage(ts).MarkFalse(TriggerConditionDependency, reason, messageFormat, messageA...)
}

func (ts *TriggerStatus) MarkDependencyUnknown(reason, messageFormat string, messageA ...interface{}) {
	triggerCondSet.Manage(ts).MarkUnknown(TriggerConditionDependency, reason, messageFormat, messageA...)
}

func (ts *TriggerStatus) PropagateDependencyStatus(ks *duckv1.KResource) {
	kc := ks.Status.GetCondition(apis.ConditionReady)
	if kc == nil {
		ts.MarkDependencyUnknown("DependencyUnknown", "The condition of Dependency is nil")
		return
	}
	if kc.IsTrue() {
		ts.MarkDependencySucceeded()
	} else {
		msg := kc.Message
		if kc.IsUnknown() {
			ts.MarkDependencyUnknown("DependencyUnknown", "The status of Dependency is Unknown: %s", msg)
		} else {
			ts.MarkDependencyFailed("DependencyFalse", "The status of Dependency is False: %s", msg)
		}
	}
}
