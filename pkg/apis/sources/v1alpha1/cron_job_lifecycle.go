/*
Copyright 2019 The Knative Authors

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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"knative.dev/pkg/apis"

	"knative.dev/eventing/pkg/apis/duck"
)

const (
	// CronJobConditionReady has status True when the CronJobSource is ready to send events.
	CronJobConditionReady = apis.ConditionReady

	// CronJobConditionValidSchedule has status True when the CronJobSource has been configured with a valid schedule.
	CronJobConditionValidSchedule apis.ConditionType = "ValidSchedule"

	// CronJobConditionSinkProvided has status True when the CronJobSource has been configured with a sink target.
	CronJobConditionSinkProvided apis.ConditionType = "SinkProvided"

	// CronJobConditionDeployed has status True when the CronJobSource has had it's receive adapter deployment created.
	CronJobConditionDeployed apis.ConditionType = "Deployed"

	// CronJobConditionEventTypeProvided has status True when the CronJobSource has been configured with its event type.
	CronJobConditionEventTypeProvided apis.ConditionType = "EventTypeProvided"

	// CronJobConditionResources is True when the resources listed for the CronJobSource have been properly
	// parsed and match specified syntax for resource quantities
	CronJobConditionResources apis.ConditionType = "ResourcesCorrect"
)

var cronJobSourceCondSet = apis.NewLivingConditionSet(
	CronJobConditionValidSchedule,
	CronJobConditionSinkProvided,
	CronJobConditionDeployed)

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *CronJobSourceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return cronJobSourceCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *CronJobSourceStatus) IsReady() bool {
	return cronJobSourceCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *CronJobSourceStatus) InitializeConditions() {
	cronJobSourceCondSet.Manage(s).InitializeConditions()
}

// TODO: this is a bad method name, change it.
// MarkSchedule sets the condition that the source has a valid schedule configured.
func (s *CronJobSourceStatus) MarkSchedule() {
	cronJobSourceCondSet.Manage(s).MarkTrue(CronJobConditionValidSchedule)
}

// MarkInvalidSchedule sets the condition that the source does not have a valid schedule configured.
func (s *CronJobSourceStatus) MarkInvalidSchedule(reason, messageFormat string, messageA ...interface{}) {
	cronJobSourceCondSet.Manage(s).MarkFalse(CronJobConditionValidSchedule, reason, messageFormat, messageA...)
}

// MarkSink sets the condition that the source has a sink configured.
func (s *CronJobSourceStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		cronJobSourceCondSet.Manage(s).MarkTrue(CronJobConditionSinkProvided)
	} else {
		cronJobSourceCondSet.Manage(s).MarkUnknown(CronJobConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

// MarkSinkWarnDeprecated sets the condition that the source has a sink configured and warns ref is deprecated.
func (s *CronJobSourceStatus) MarkSinkWarnRefDeprecated(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		c := apis.Condition{
			Type:     CronJobConditionSinkProvided,
			Status:   corev1.ConditionTrue,
			Severity: apis.ConditionSeverityError,
			Message:  "Using deprecated object ref fields when specifying spec.sink. Update to spec.sink.ref. These will be removed in 0.11.",
		}
		apiserverCondSet.Manage(s).SetCondition(c)
	} else {
		apiserverCondSet.Manage(s).MarkUnknown(CronJobConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *CronJobSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	cronJobSourceCondSet.Manage(s).MarkFalse(CronJobConditionSinkProvided, reason, messageFormat, messageA...)
}

// PropagateDeploymentAvailability uses the availability of the provided Deployment to determine if
// CronJobConditionDeployed should be marked as true or false.
func (s *CronJobSourceStatus) PropagateDeploymentAvailability(d *appsv1.Deployment) {
	if duck.DeploymentIsAvailable(&d.Status, false) {
		cronJobSourceCondSet.Manage(s).MarkTrue(CronJobConditionDeployed)
	} else {
		// I don't know how to propagate the status well, so just give the name of the Deployment
		// for now.
		cronJobSourceCondSet.Manage(s).MarkFalse(CronJobConditionDeployed, "DeploymentUnavailable", "The Deployment '%s' is unavailable.", d.Name)
	}
}

// MarkEventType sets the condition that the source has set its event type.
func (s *CronJobSourceStatus) MarkEventType() {
	cronJobSourceCondSet.Manage(s).MarkTrue(CronJobConditionEventTypeProvided)
}

// MarkNoEventType sets the condition that the source does not its event type configured.
func (s *CronJobSourceStatus) MarkNoEventType(reason, messageFormat string, messageA ...interface{}) {
	cronJobSourceCondSet.Manage(s).MarkFalse(CronJobConditionEventTypeProvided, reason, messageFormat, messageA...)
}

// MarkResourcesCorrect sets the condition that the source resources are properly parsable quantities
func (s *CronJobSourceStatus) MarkResourcesCorrect() {
	cronJobSourceCondSet.Manage(s).MarkTrue(CronJobConditionResources)
}

// MarkResourcesIncorrect sets the condition that the source resources are not properly parsable quantities
func (s *CronJobSourceStatus) MarkResourcesIncorrect(reason, messageFormat string, messageA ...interface{}) {
	cronJobSourceCondSet.Manage(s).MarkFalse(CronJobConditionResources, reason, messageFormat, messageA...)
}
