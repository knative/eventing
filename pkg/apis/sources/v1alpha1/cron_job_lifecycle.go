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
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
)

const (
	// CronJobConditionReady has status True when the CronJobSource is ready to send events.
	CronJobConditionReady = duckv1alpha1.ConditionReady

	// CronJobConditionValidSchedule has status True when the CronJobSource has been configured with a valid schedule.
	CronJobConditionValidSchedule duckv1alpha1.ConditionType = "ValidSchedule"

	// CronJobConditionSinkProvided has status True when the CronJobSource has been configured with a sink target.
	CronJobConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"

	// CronJobConditionDeployed has status True when the CronJobSource has had it's receive adapter deployment created.
	CronJobConditionDeployed duckv1alpha1.ConditionType = "Deployed"

	// CronJobConditionEventTypeProvided has status True when the CronJobSource has been configured with its event type.
	CronJobConditionEventTypeProvided duckv1alpha1.ConditionType = "EventTypeProvided"
)

var cronJobSourceCondSet = duckv1alpha1.NewLivingConditionSet(
	CronJobConditionValidSchedule,
	CronJobConditionSinkProvided,
	CronJobConditionDeployed)

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *CronJobSourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
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

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *CronJobSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	cronJobSourceCondSet.Manage(s).MarkFalse(CronJobConditionSinkProvided, reason, messageFormat, messageA...)
}

// MarkDeployed sets the condition that the source has been deployed.
func (s *CronJobSourceStatus) MarkDeployed() {
	cronJobSourceCondSet.Manage(s).MarkTrue(CronJobConditionDeployed)
}

// MarkDeploying sets the condition that the source is deploying.
func (s *CronJobSourceStatus) MarkDeploying(reason, messageFormat string, messageA ...interface{}) {
	cronJobSourceCondSet.Manage(s).MarkUnknown(CronJobConditionDeployed, reason, messageFormat, messageA...)
}

// MarkNotDeployed sets the condition that the source has not been deployed.
func (s *CronJobSourceStatus) MarkNotDeployed(reason, messageFormat string, messageA ...interface{}) {
	cronJobSourceCondSet.Manage(s).MarkFalse(CronJobConditionDeployed, reason, messageFormat, messageA...)
}

// MarkEventType sets the condition that the source has set its event type.
func (s *CronJobSourceStatus) MarkEventType() {
	cronJobSourceCondSet.Manage(s).MarkTrue(CronJobConditionEventTypeProvided)
}

// MarkNoEventType sets the condition that the source does not its event type configured.
func (s *CronJobSourceStatus) MarkNoEventType(reason, messageFormat string, messageA ...interface{}) {
	cronJobSourceCondSet.Manage(s).MarkFalse(CronJobConditionEventTypeProvided, reason, messageFormat, messageA...)
}
