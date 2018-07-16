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

// NewBusCondition creates a new bus condition with the provided values and both times set to now().
func NewBusCondition(condType v1alpha1.BusConditionType, status v1.ConditionStatus, reason, message string) *v1alpha1.BusCondition {
	return &v1alpha1.BusCondition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     meta_v1.Now(),
		LastTransitionTime: meta_v1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// GetBusCondition returns the bus condition with the provided type.
func GetBusCondition(status v1alpha1.BusStatus, condType v1alpha1.BusConditionType) *v1alpha1.BusCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

// SetBusCondition updates the bus status to include the provided condition. If the condition that
// we are about to add already exists and has the same status and reason then no update happens.
func SetBusCondition(status *v1alpha1.BusStatus, condition v1alpha1.BusCondition) {
	currentCond := GetBusCondition(*status, condition.Type)
	if currentCond != nil && currentCond.Status == condition.Status && currentCond.Reason == condition.Reason {
		return
	}
	// Do not update lastTransitionTime if the status of the condition doesn't change.
	if currentCond != nil && currentCond.Status == condition.Status {
		condition.LastTransitionTime = currentCond.LastTransitionTime
	}
	newConditions := filterOutBusCondition(status.Conditions, condition.Type)
	status.Conditions = append(newConditions, condition)
}

// RemoveBusCondition removes the bus condition with the provided type.
func RemoveBusCondition(status *v1alpha1.BusStatus, condType v1alpha1.BusConditionType) {
	status.Conditions = filterOutBusCondition(status.Conditions, condType)
}

// ConsolidateBusCondition computes and sets the overall "Ready" condition of the bus
// given all other sub-conditions.
func ConsolidateBusCondition(bus *v1alpha1.Bus) {
	dispatching := GetBusCondition(bus.Status, v1alpha1.BusDispatching)
	provisioning := GetBusCondition(bus.Status, v1alpha1.BusProvisioning)
	serviceable := GetBusCondition(bus.Status, v1alpha1.BusServiceable)
	needsProvitioner := bus.Spec.Provisioner != nil

	var cond *v1alpha1.BusCondition

	if dispatching != nil && dispatching.Status == v1.ConditionTrue &&
		serviceable != nil && serviceable.Status == v1.ConditionTrue &&
		((provisioning != nil && provisioning.Status == v1.ConditionTrue) || !needsProvitioner) {
		cond = NewBusCondition(v1alpha1.BusReady, v1.ConditionTrue, "", "")
	} else {
		cond = NewBusCondition(v1alpha1.BusReady, v1.ConditionFalse, "", "")
	}
	SetBusCondition(&bus.Status, *cond)
}

// IsBusReady returns whether all readiness conditions of a bus are met, as a boolean.
func IsBusReady(status *v1alpha1.BusStatus) bool {
	c := GetBusCondition(*status, v1alpha1.BusReady)
	return c != nil && c.Status == v1.ConditionTrue
}

// filterOutBusCondition returns a new slice of bus conditions without conditions with the provided type.
func filterOutBusCondition(conditions []v1alpha1.BusCondition, condType v1alpha1.BusConditionType) []v1alpha1.BusCondition {
	var newConditions []v1alpha1.BusCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}
