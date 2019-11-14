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

// IsReady returns true if the resource is ready overall.
func (ts *TriggerStatus) IsReady() bool {
	return triggerCondSet.Manage(ts).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ts *TriggerStatus) InitializeConditions() {
	triggerCondSet.Manage(ts).InitializeConditions()
}

func (ts *TriggerStatus) PropagateBrokerStatus(bs *BrokerStatus) {
	if bs.IsReady() {
		triggerCondSet.Manage(ts).MarkTrue(TriggerConditionBroker)
	} else {
		msg := "nil"
		if bc := brokerCondSet.Manage(bs).GetCondition(BrokerConditionReady); bc != nil {
			msg = bc.Message
		}
		ts.MarkBrokerFailed("BrokerNotReady", "Broker is not ready: %s", msg)
	}
}

func (ts *TriggerStatus) MarkBrokerFailed(reason, messageFormat string, messageA ...interface{}) {
	triggerCondSet.Manage(ts).MarkFalse(TriggerConditionBroker, reason, messageFormat, messageA...)
}

func (ts *TriggerStatus) PropagateSubscriptionStatus(ss *messagingv1alpha1.SubscriptionStatus) {
	if ss.IsReady() {
		triggerCondSet.Manage(ts).MarkTrue(TriggerConditionSubscribed)
	} else {
		msg := "nil"
		if sc := ss.Status.GetCondition(messagingv1alpha1.SubscriptionConditionReady); sc != nil {
			msg = sc.Message
		}
		ts.MarkNotSubscribed("SubscriptionNotReady", "Subscription is not ready: %s", msg)
	}
}

func (ts *TriggerStatus) MarkNotSubscribed(reason, messageFormat string, messageA ...interface{}) {
	triggerCondSet.Manage(ts).MarkFalse(TriggerConditionSubscribed, reason, messageFormat, messageA...)
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
	if kc != nil && kc.IsTrue() {
		ts.MarkDependencySucceeded()
	} else {
		msg := "nil"
		if kc != nil {
			msg = kc.Message
		}
		ts.MarkDependencyFailed("DependencyNotReady", "Dependency is not ready: %s", msg)
	}
}
