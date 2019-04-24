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

// AwsSqsSource is the Schema for the AWS SQS API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:categories=all,knative,eventing,sources
type AwsSqsSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AwsSqsSourceSpec   `json:"spec,omitempty"`
	Status AwsSqsSourceStatus `json:"status,omitempty"`
}

// Check that AwsSqsSource can be validated and can be defaulted.
var _ runtime.Object = (*AwsSqsSource)(nil)

// Check that AwsSqsSource implements the Conditions duck type.
var _ = duck.VerifyType(&AwsSqsSource{}, &duckv1alpha1.Conditions{})

// AwsSqsSourceSpec defines the desired state of the source.
type AwsSqsSourceSpec struct {
	// QueueURL of the SQS queue that we will poll from.
	QueueURL string `json:"queueUrl"`

	// AwsCredsSecret is the credential to use to poll the AWS SQS
	AwsCredsSecret corev1.SecretKeySelector `json:"awsCredsSecret,omitempty"`

	// Sink is a reference to an object that will resolve to a domain name to
	// use as the sink.  This is where events will be received.
	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`

	// ServiceAccoutName is the name of the ServiceAccount that will be used to
	// run the Receive Adapter Deployment.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

const (
	// AwsSqsSourceConditionReady has status True when the source is
	// ready to send events.
	AwsSqsSourceConditionReady = duckv1alpha1.ConditionReady

	// AwsSqsSourceConditionSinkProvided has status True when the
	// AwsSqsSource has been configured with a sink target.
	AwsSqsSourceConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"

	// AwsSqsSourceConditionDeployed has status True when the
	// AwsSqsSource has had it's receive adapter deployment created.
	AwsSqsSourceConditionDeployed duckv1alpha1.ConditionType = "Deployed"
)

var condSet = duckv1alpha1.NewLivingConditionSet(
	AwsSqsSourceConditionReady,
	AwsSqsSourceConditionSinkProvided,
	AwsSqsSourceConditionDeployed)

// AwsSqsSourceStatus defines the observed state of the source.
type AwsSqsSourceStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// SinkURI is the current active sink URI that has been configured for the source.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *AwsSqsSourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return condSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *AwsSqsSourceStatus) IsReady() bool {
	return condSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *AwsSqsSourceStatus) InitializeConditions() {
	condSet.Manage(s).InitializeConditions()
}

// MarkSink sets the condition that the source has a sink configured.
func (s *AwsSqsSourceStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		condSet.Manage(s).MarkTrue(AwsSqsSourceConditionSinkProvided)
	} else {
		condSet.Manage(s).MarkUnknown(AwsSqsSourceConditionSinkProvided,
			"SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *AwsSqsSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	condSet.Manage(s).MarkFalse(AwsSqsSourceConditionSinkProvided, reason, messageFormat, messageA...)
}

// MarkDeployed sets the condition that the source has been deployed.
func (s *AwsSqsSourceStatus) MarkDeployed() {
	condSet.Manage(s).MarkTrue(AwsSqsSourceConditionDeployed)
}

// MarkDeploying sets the condition that the source is deploying.
func (s *AwsSqsSourceStatus) MarkDeploying(reason, messageFormat string, messageA ...interface{}) {
	condSet.Manage(s).MarkUnknown(AwsSqsSourceConditionDeployed, reason, messageFormat, messageA...)
}

// MarkNotDeployed sets the condition that the source has not been deployed.
func (s *AwsSqsSourceStatus) MarkNotDeployed(reason, messageFormat string, messageA ...interface{}) {
	condSet.Manage(s).MarkFalse(AwsSqsSourceConditionDeployed, reason, messageFormat, messageA...)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AwsSqsSourceList contains a list of AwsSqsSource
type AwsSqsSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AwsSqsSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AwsSqsSource{}, &AwsSqsSourceList{})
}
