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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/pkg/apis"
)

const (
	// IntegrationSourceConditionReady has status True when the IntegrationSource is ready to send events.
	IntegrationSourceConditionReady = apis.ConditionReady

	// IntegrationSourceConditionContainerSourceReady has status True when the IntegrationSource's ContainerSource is ready.
	IntegrationSourceConditionContainerSourceReady apis.ConditionType = "ContainerSourceReady"
)

var IntegrationCondSet = apis.NewLivingConditionSet(
	IntegrationSourceConditionContainerSourceReady,
)

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*IntegrationSource) GetConditionSet() apis.ConditionSet {
	return IntegrationCondSet
}

// GetTopLevelCondition returns the top level condition.
func (s *IntegrationSourceStatus) GetTopLevelCondition() *apis.Condition {
	return IntegrationCondSet.Manage(s).GetTopLevelCondition()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *IntegrationSourceStatus) InitializeConditions() {
	IntegrationCondSet.Manage(s).InitializeConditions()
}

func (iss *IntegrationSourceStatus) IsReady() bool {
	return IntegrationCondSet.Manage(iss).IsHappy()
}

func (s *IntegrationSourceStatus) PropagateContainerSourceStatus(status *v1.ContainerSourceStatus) {
	// ContainerSource status has all we need, hence deep copy it.
	s.SourceStatus = *status.SourceStatus.DeepCopy()

	cond := status.GetCondition(apis.ConditionReady)
	switch {
	case cond == nil:
		IntegrationCondSet.Manage(s).MarkUnknown(IntegrationSourceConditionContainerSourceReady, "", "")
	case cond.Status == corev1.ConditionTrue:
		IntegrationCondSet.Manage(s).MarkTrue(IntegrationSourceConditionContainerSourceReady)
	case cond.Status == corev1.ConditionFalse:
		IntegrationCondSet.Manage(s).MarkFalse(IntegrationSourceConditionContainerSourceReady, cond.Reason, cond.Message)
	case cond.Status == corev1.ConditionUnknown:
		IntegrationCondSet.Manage(s).MarkUnknown(IntegrationSourceConditionContainerSourceReady, cond.Reason, cond.Message)
	default:
		IntegrationCondSet.Manage(s).MarkUnknown(IntegrationSourceConditionContainerSourceReady, cond.Reason, cond.Message)
	}

	// Propagate ContainerSources AuthStatus to IntegrationSources AuthStatus
	s.Auth = status.Auth
}
