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
 */

package util

import (
	"k8s.io/api/core/v1"
	"github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewChannelCondition creates a new channel condition with the provided values and both times set to now().
func NewChannelCondition(condType v1alpha1.ChannelConditionType, status v1.ConditionStatus, reason, message string) *v1alpha1.ChannelCondition {
	return &v1alpha1.ChannelCondition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     meta_v1.Now(),
		LastTransitionTime: meta_v1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// GetChannelCondition returns the channel condition with the provided type.
func GetChannelCondition(status v1alpha1.ChannelStatus, condType v1alpha1.ChannelConditionType) *v1alpha1.ChannelCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

// SetChannelCondition updates the channel status to include the provided condition. If the condition that
// we are about to add already exists and has the same status and reason then no update happens.
func SetChannelCondition(status *v1alpha1.ChannelStatus, condition v1alpha1.ChannelCondition) {
	currentCond := GetChannelCondition(*status, condition.Type)
	if currentCond != nil && currentCond.Status == condition.Status && currentCond.Reason == condition.Reason {
		return
	}
	// Do not update lastTransitionTime if the status of the condition doesn't change.
	if currentCond != nil && currentCond.Status == condition.Status {
		condition.LastTransitionTime = currentCond.LastTransitionTime
	}
	newConditions := filterOutCondition(status.Conditions, condition.Type)
	status.Conditions = append(newConditions, condition)
}

// RemoveChannelCondition removes the channel condition with the provided type.
func RemoveChannelCondition(status *v1alpha1.ChannelStatus, condType v1alpha1.ChannelConditionType) {
	status.Conditions = filterOutCondition(status.Conditions, condType)
}

// filterOutCondition returns a new slice of channel conditions without conditions with the provided type.
func filterOutCondition(conditions []v1alpha1.ChannelCondition, condType v1alpha1.ChannelConditionType) []v1alpha1.ChannelCondition {
	var newConditions []v1alpha1.ChannelCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}
