/*
Copyright 2020 The Knative Authors

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

package v1alpha2

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/eventing/pkg/apis/duck"
	"knative.dev/pkg/apis"
)

const (
	// ContainerSourceConditionReady has status True when the ContainerSource is ready to send events.
	ContainerSourceConditionReady = apis.ConditionReady

	// ContainerSourceConditionSinkBindingReady has status True when the ContainerSource's SinkBinding is ready.
	ContainerSourceConditionSinkBindingReady apis.ConditionType = "SinkBindingReady"

	// ContainerSourceConditionReceiveAdapterReady has status True when the ContainerSource's ReceiveAdapter is ready.
	ContainerSourceConditionReceiveAdapterReady apis.ConditionType = "ReceiveAdapterReady"
)

var containerCondSet = apis.NewLivingConditionSet(
	ContainerSourceConditionSinkBindingReady,
	ContainerSourceConditionReceiveAdapterReady,
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *ContainerSourceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return containerCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *ContainerSourceStatus) IsReady() bool {
	return containerCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *ContainerSourceStatus) InitializeConditions() {
	containerCondSet.Manage(s).InitializeConditions()
}

// PropagateSinkBindingStatus uses the availability of the provided Deployment to determine if
// ContainerSourceConditionSinkBindingReady should be marked as true, false or unknown.
func (s *ContainerSourceStatus) PropagateSinkBindingStatus(status *SinkBindingStatus) {
	// Do not copy conditions nor observedGeneration
	conditions := s.Conditions
	observedGeneration := s.ObservedGeneration
	s.SourceStatus = status.SourceStatus
	s.Conditions = conditions
	s.ObservedGeneration = observedGeneration

	cond := status.GetCondition(apis.ConditionReady)
	switch {
	case cond == nil:
		containerCondSet.Manage(s).MarkUnknown(ContainerSourceConditionSinkBindingReady, "", "")
	case cond.Status == corev1.ConditionTrue:
		containerCondSet.Manage(s).MarkTrue(ContainerSourceConditionSinkBindingReady)
	case cond.Status == corev1.ConditionFalse:
		containerCondSet.Manage(s).MarkFalse(ContainerSourceConditionSinkBindingReady, cond.Reason, cond.Message)
	case cond.Status == corev1.ConditionUnknown:
		containerCondSet.Manage(s).MarkUnknown(ContainerSourceConditionSinkBindingReady, cond.Reason, cond.Message)
	default:
		containerCondSet.Manage(s).MarkUnknown(ContainerSourceConditionSinkBindingReady, cond.Reason, cond.Message)
	}
}

// PropagateReceiveAdapterStatus uses the availability of the provided Deployment to determine if
// ContainerSourceConditionReceiveAdapterReady should be marked as true or false.
func (s *ContainerSourceStatus) PropagateReceiveAdapterStatus(d *appsv1.Deployment) {
	if duck.DeploymentIsAvailable(&d.Status, false) {
		containerCondSet.Manage(s).MarkTrue(ContainerSourceConditionReceiveAdapterReady)
	} else {
		// I don't know how to propagate the status well, so just give the name of the Deployment
		// for now.
		containerCondSet.Manage(s).MarkFalse(ContainerSourceConditionReceiveAdapterReady, "DeploymentUnavailable", "The Deployment '%s' is unavailable.", d.Name)
	}
}
