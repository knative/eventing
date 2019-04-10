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

const (
	// SubscriptionConditionReady has status True when all subconditions below have been set to True.
	SubscriptionConditionReady = duckv1alpha1.ConditionReady
	// SubscriptionConditionReferencesResolved has status True when all the specified references have been successfully
	// resolved.
	SubscriptionConditionReferencesResolved duckv1alpha1.ConditionType = "Resolved"

	// SubscriptionConditionChannelReady has status True when controller has successfully added a
	// subscription to the spec.channel resource.
	SubscriptionConditionChannelReady duckv1alpha1.ConditionType = "ChannelReady"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (ss *SubscriptionStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return subCondSet.Manage(ss).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (ss *SubscriptionStatus) IsReady() bool {
	return subCondSet.Manage(ss).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ss *SubscriptionStatus) InitializeConditions() {
	subCondSet.Manage(ss).InitializeConditions()
}

// MarkReferencesResolved sets the ReferencesResolved condition to True state.
func (ss *SubscriptionStatus) MarkReferencesResolved() {
	subCondSet.Manage(ss).MarkTrue(SubscriptionConditionReferencesResolved)
}

// MarkChannelReady sets the ChannelReady condition to True state.
func (ss *SubscriptionStatus) MarkChannelReady() {
	subCondSet.Manage(ss).MarkTrue(SubscriptionConditionChannelReady)
}
