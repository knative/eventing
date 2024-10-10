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

	// IntegrationSourceConditionSinkProvided has status True when the ApiServerSource has been configured with a sink target.
	IntegrationSourceConditionSinkProvided apis.ConditionType = "SinkProvided"

	// IntegrationSourceConditionContainerSourceReady has status True when the IntegrationSource's ContainerSource is ready.
	IntegrationSourceConditionContainerSourceReady apis.ConditionType = "ContainerSourceReady"
)

var IntegrationCondSet = apis.NewLivingConditionSet(
	IntegrationSourceConditionContainerSourceReady,
	//	IntegrationSourceConditionSinkProvided,
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

// / GetCondition returns the condition currently associated with the given type, or nil.
func (i *IntegrationSourceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return IntegrationCondSet.Manage(i).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (i *IntegrationSourceStatus) IsReady() bool {
	return IntegrationCondSet.Manage(i).IsHappy()
}

func (i *IntegrationSourceStatus) MarkSink(uri *apis.URL) {
	i.SinkURI = uri
	if len(uri.String()) > 0 {
		IntegrationCondSet.Manage(i).MarkTrue(IntegrationSourceConditionSinkProvided)
	} else {
		IntegrationCondSet.Manage(i).MarkFalse(IntegrationSourceConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

func (iss *IntegrationSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	IntegrationCondSet.Manage(iss).MarkFalse(IntegrationSourceConditionSinkProvided, reason, messageFormat, messageA...)
}

func (s *IntegrationSourceStatus) PropagateContainerSourceStatus(status *v1.ContainerSourceStatus) {
	// Do not copy conditions nor observedGeneration
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

	// Propagate SinkBindings AuthStatus to containersources AuthStatus
	s.Auth = status.Auth
}
