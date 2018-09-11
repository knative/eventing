/*
 * Copyright 2018 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package util

import (
	"github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewSubscriptionCondition creates a new subscription condition with the provided values and both times set to now().
func NewSubscriptionCondition(condType v1alpha1.SubscriptionConditionType, status v1.ConditionStatus, reason, message string) *v1alpha1.SubscriptionCondition {
	return &v1alpha1.SubscriptionCondition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     meta_v1.Now(),
		LastTransitionTime: meta_v1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// GetSubscriptionCondition returns the subscription condition with the provided type.
func GetSubscriptionCondition(status v1alpha1.SubscriptionStatus, condType v1alpha1.SubscriptionConditionType) *v1alpha1.SubscriptionCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

// SetSubscriptionCondition updates the subscription status to include the provided condition. If the condition that
// we are about to add already exists and has the same status and reason then no update happens.
func SetSubscriptionCondition(status *v1alpha1.SubscriptionStatus, condition v1alpha1.SubscriptionCondition) {
	currentCond := GetSubscriptionCondition(*status, condition.Type)
	if currentCond != nil && currentCond.Status == condition.Status && currentCond.Reason == condition.Reason {
		return
	}
	// Do not update lastTransitionTime if the status of the condition doesn't change.
	if currentCond != nil && currentCond.Status == condition.Status {
		condition.LastTransitionTime = currentCond.LastTransitionTime
	}
	newConditions := filterOutSubscriptionCondition(status.Conditions, condition.Type)
	status.Conditions = append(newConditions, condition)
}

// RemoveSubscriptionCondition removes the subscription condition with the provided type.
func RemoveSubscriptionCondition(status *v1alpha1.SubscriptionStatus, condType v1alpha1.SubscriptionConditionType) {
	status.Conditions = filterOutSubscriptionCondition(status.Conditions, condType)
}

// filterOutSubscriptionCondition returns a new slice of subscription conditions without conditions with the provided type.
func filterOutSubscriptionCondition(conditions []v1alpha1.SubscriptionCondition, condType v1alpha1.SubscriptionConditionType) []v1alpha1.SubscriptionCondition {
	var newConditions []v1alpha1.SubscriptionCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}
