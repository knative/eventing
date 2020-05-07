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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"

	"knative.dev/eventing/pkg/apis/duck"
)

const (
	// PingSourceConditionReady has status True when the PingSource is ready to send events.
	PingSourceConditionReady = apis.ConditionReady

	// PingSourceConditionValidSchedule has status True when the PingSource has been configured with a valid schedule.
	PingSourceConditionValidSchedule apis.ConditionType = "ValidSchedule"

	// PingSourceConditionSinkProvided has status True when the PingSource has been configured with a sink target.
	PingSourceConditionSinkProvided apis.ConditionType = "SinkProvided"

	// PingSourceConditionDeployed has status True when the PingSource has had it's receive adapter deployment created.
	PingSourceConditionDeployed apis.ConditionType = "Deployed"

	// PingSourceConditionResources is True when the resources listed for the PingSource have been properly
	// parsed and match specified syntax for resource quantities
	PingSourceConditionResources apis.ConditionType = "ResourcesCorrect"
)

var PingSourceCondSet = apis.NewLivingConditionSet(
	PingSourceConditionValidSchedule,
	PingSourceConditionSinkProvided,
	PingSourceConditionDeployed)

const (
	// PingSourceEventType is the default PingSource CloudEvent type.
	PingSourceEventType = "dev.knative.sources.ping"
)

// PingSourceSource returns the PingSource CloudEvent source.
func PingSourceSource(namespace, name string) string {
	return fmt.Sprintf("/apis/v1/namespaces/%s/pingsources/%s", namespace, name)
}

// GetUntypedSpec returns the spec of the PingSource.
func (s *PingSource) GetUntypedSpec() interface{} {
	return s.Spec
}

// GetGroupVersionKind returns the GroupVersionKind.
func (s *PingSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("PingSource")
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *PingSourceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return PingSourceCondSet.Manage(s).GetCondition(t)
}

// GetTopLevelCondition returns the top level Condition.
func (ps *PingSourceStatus) GetTopLevelCondition() *apis.Condition {
	return PingSourceCondSet.Manage(ps).GetTopLevelCondition()
}

// IsReady returns true if the resource is ready overall.
func (s *PingSourceStatus) IsReady() bool {
	return PingSourceCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *PingSourceStatus) InitializeConditions() {
	PingSourceCondSet.Manage(s).InitializeConditions()
}

// TODO: this is a bad method name, change it.
// MarkSchedule sets the condition that the source has a valid schedule configured.
func (s *PingSourceStatus) MarkSchedule() {
	PingSourceCondSet.Manage(s).MarkTrue(PingSourceConditionValidSchedule)
}

// MarkInvalidSchedule sets the condition that the source does not have a valid schedule configured.
func (s *PingSourceStatus) MarkInvalidSchedule(reason, messageFormat string, messageA ...interface{}) {
	PingSourceCondSet.Manage(s).MarkFalse(PingSourceConditionValidSchedule, reason, messageFormat, messageA...)
}

// MarkSink sets the condition that the source has a sink configured.
func (s *PingSourceStatus) MarkSink(uri *apis.URL) {
	// TODO: Update sources to use MarkSink(url.URL or apis.URI)
	s.SinkURI = uri
	if uri != nil {
		PingSourceCondSet.Manage(s).MarkTrue(PingSourceConditionSinkProvided)
	} else {
		PingSourceCondSet.Manage(s).MarkFalse(PingSourceConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *PingSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	PingSourceCondSet.Manage(s).MarkFalse(PingSourceConditionSinkProvided, reason, messageFormat, messageA...)
}

// PropagateDeploymentAvailability uses the availability of the provided Deployment to determine if
// PingSourceConditionDeployed should be marked as true or false.
func (s *PingSourceStatus) PropagateDeploymentAvailability(d *appsv1.Deployment) {
	if duck.DeploymentIsAvailable(&d.Status, false) {
		PingSourceCondSet.Manage(s).MarkTrue(PingSourceConditionDeployed)
	} else {
		// I don't know how to propagate the status well, so just give the name of the Deployment
		// for now.
		PingSourceCondSet.Manage(s).MarkFalse(PingSourceConditionDeployed, "DeploymentUnavailable", "The Deployment '%s' is unavailable.", d.Name)
	}
}

// MarkResourcesCorrect sets the condition that the source resources are properly parsable quantities
func (s *PingSourceStatus) MarkResourcesCorrect() {
	PingSourceCondSet.Manage(s).MarkTrue(PingSourceConditionResources)
}

// MarkResourcesIncorrect sets the condition that the source resources are not properly parsable quantities
func (s *PingSourceStatus) MarkResourcesIncorrect(reason, messageFormat string, messageA ...interface{}) {
	PingSourceCondSet.Manage(s).MarkFalse(PingSourceConditionResources, reason, messageFormat, messageA...)
}
