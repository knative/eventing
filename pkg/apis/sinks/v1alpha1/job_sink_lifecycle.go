/*
Copyright 2024 The Knative Authors

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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"

	"knative.dev/eventing/pkg/apis/sinks"
)

const (
	// JobSinkConditionReady has status True when the JobSink is ready to send events.
	JobSinkConditionReady = apis.ConditionReady

	JobSinkConditionAddressable apis.ConditionType = "Addressable"
)

var JobSinkCondSet = apis.NewLivingConditionSet(
	JobSinkConditionAddressable,
)

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*JobSink) GetConditionSet() apis.ConditionSet {
	return JobSinkCondSet
}

// GetUntypedSpec returns the spec of the JobSink.
func (sink *JobSink) GetUntypedSpec() interface{} {
	return sink.Spec
}

// GetGroupVersionKind returns the GroupVersionKind.
func (sink *JobSink) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("JobSink")
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *JobSinkStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return JobSinkCondSet.Manage(s).GetCondition(t)
}

// GetTopLevelCondition returns the top level Condition.
func (ps *JobSinkStatus) GetTopLevelCondition() *apis.Condition {
	return JobSinkCondSet.Manage(ps).GetTopLevelCondition()
}

// IsReady returns true if the resource is ready overall.
func (s *JobSinkStatus) IsReady() bool {
	return JobSinkCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *JobSinkStatus) InitializeConditions() {
	JobSinkCondSet.Manage(s).InitializeConditions()
}

func (e *JobSink) SetJobStatusSelector() {
	if e.Spec.Job != nil {
		e.Status.JobStatus.Selector = fmt.Sprintf("%s=%s", sinks.JobSinkNameLabel, e.GetName())
	}
}
