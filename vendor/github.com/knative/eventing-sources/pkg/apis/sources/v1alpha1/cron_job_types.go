/*
Copyright 2018 The Knative Authors

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
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CronJobSource is the Schema for the cronjobsources API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:categories=all,knative,eventing,sources
type CronJobSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CronJobSourceSpec   `json:"spec,omitempty"`
	Status CronJobSourceStatus `json:"status,omitempty"`
}

// Check that CronJobSource can be validated and can be defaulted.
var _ runtime.Object = (*CronJobSource)(nil)

// Check that CronJobSource implements the Conditions duck type.
var _ = duck.VerifyType(&CronJobSource{}, &duckv1alpha1.Conditions{})

// CronJobSourceSpec defines the desired state of the CronJobSource.
type CronJobSourceSpec struct {

	// Schedule is the cronjob schedule.
	// +required
	Schedule string `json:"schedule"`

	// Data is the data posted to the target function.
	Data string `json:"data,omitempty"`

	// Sink is a reference to an object that will resolve to a domain name to use as the sink.
	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`

	// ServiceAccoutName is the name of the ServiceAccount that will be used to run the Receive
	// Adapter Deployment.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

const (
	// CronJobConditionReady has status True when the CronJobSource is ready to send events.
	CronJobConditionReady = duckv1alpha1.ConditionReady

	// CronJobConditionValidSchedule has status True when the CronJobSource has been configured with a valid schedule.
	CronJobConditionValidSchedule duckv1alpha1.ConditionType = "ValidSchedule"

	// CronJobConditionSinkProvided has status True when the CronJobSource has been configured with a sink target.
	CronJobConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"

	// CronJobConditionDeployed has status True when the CronJobSource has had it's receive adapter deployment created.
	CronJobConditionDeployed duckv1alpha1.ConditionType = "Deployed"
)

var cronJobSourceCondSet = duckv1alpha1.NewLivingConditionSet(
	CronJobConditionValidSchedule,
	CronJobConditionSinkProvided,
	CronJobConditionDeployed)

// CronJobSourceStatus defines the observed state of CronJobSource.
type CronJobSourceStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// SinkURI is the current active sink URI that has been configured for the CronJobSource.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CronJobSourceList contains a list of CronJobSources.
type CronJobSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CronJobSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CronJobSource{}, &CronJobSourceList{})
}
