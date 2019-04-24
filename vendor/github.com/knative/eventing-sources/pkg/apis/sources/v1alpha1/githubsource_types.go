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

// Check that GitHubSource can be validated and can be defaulted.
var _ runtime.Object = (*GitHubSource)(nil)

// Check that GitHubSource implements the Conditions duck type.
var _ = duck.VerifyType(&GitHubSource{}, &duckv1alpha1.Conditions{})

// GitHubSourceSpec defines the desired state of GitHubSource
// +kubebuilder:categories=all,knative,eventing,sources
type GitHubSourceSpec struct {
	// ServiceAccountName holds the name of the Kubernetes service account
	// as which the underlying K8s resources should be run. If unspecified
	// this will default to the "default" service account for the namespace
	// in which the GitHubSource exists.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// OwnerAndRepository is the GitHub owner/org and repository to
	// receive events from. The repository may be left off to receive
	// events from an entire organization.
	// Examples:
	//  myuser/project
	//  myorganization
	// +kubebuilder:validation:MinLength=1
	OwnerAndRepository string `json:"ownerAndRepository"`

	// EventType is the type of event to receive from GitHub. These
	// correspond to the "Webhook event name" values listed at
	// https://developer.github.com/v3/activity/events/types/ - ie
	// "pull_request"
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Enum=commit_comment,create,delete,deployment,deployment_status,fork,gollum,installation,integration_installation,issue_comment,issues,label,member,membership,milestone,organization,org_block,page_build,ping,project_card,project_column,project,public,pull_request,pull_request_review,pull_request_review_comment,push,release,repository,status,team,team_add,watch
	EventTypes []string `json:"eventTypes"`

	// AccessToken is the Kubernetes secret containing the GitHub
	// access token
	AccessToken SecretValueFromSource `json:"accessToken"`

	// SecretToken is the Kubernetes secret containing the GitHub
	// secret token
	SecretToken SecretValueFromSource `json:"secretToken"`

	// Sink is a reference to an object that will resolve to a domain
	// name to use as the sink.
	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`

	// API URL if using github enterprise (default https://api.github.com)
	// +optional
	GitHubAPIURL string `json:"githubAPIURL,omitempty"`

	// Secure can be set to true to configure the webhook to use https.
	// +optional
	Secure bool `json:"secure,omitempty"`
}

// SecretValueFromSource represents the source of a secret value
type SecretValueFromSource struct {
	// The Secret key to select from.
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

const (
	// GitHubSourceEventPrefix is what all GitHub event types get
	// prefixed with when converting to CloudEvent EventType
	GitHubSourceEventPrefix = "dev.knative.source.github"
)

const (
	// GitHubSourceConditionReady has status True when the
	// GitHubSource is ready to send events.
	GitHubSourceConditionReady = duckv1alpha1.ConditionReady

	// GitHubSourceConditionSecretsProvided has status True when the
	// GitHubSource has valid secret references
	GitHubSourceConditionSecretsProvided duckv1alpha1.ConditionType = "SecretsProvided"

	// GitHubSourceConditionSinkProvided has status True when the
	// GitHubSource has been configured with a sink target.
	GitHubSourceConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"
)

var gitHubSourceCondSet = duckv1alpha1.NewLivingConditionSet(
	GitHubSourceConditionSecretsProvided,
	GitHubSourceConditionSinkProvided)

// GitHubSourceStatus defines the observed state of GitHubSource
type GitHubSourceStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// WebhookIDKey is the ID of the webhook registered with GitHub
	WebhookIDKey string `json:"webhookIDKey,omitempty"`

	// SinkURI is the current active sink URI that has been configured
	// for the GitHubSource.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *GitHubSourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return gitHubSourceCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *GitHubSourceStatus) IsReady() bool {
	return gitHubSourceCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *GitHubSourceStatus) InitializeConditions() {
	gitHubSourceCondSet.Manage(s).InitializeConditions()
}

// MarkSecrets sets the condition that the source has a valid spec
func (s *GitHubSourceStatus) MarkSecrets() {
	gitHubSourceCondSet.Manage(s).MarkTrue(GitHubSourceConditionSecretsProvided)
}

// MarkNoSecrets sets the condition that the source does not have a valid spec
func (s *GitHubSourceStatus) MarkNoSecrets(reason, messageFormat string, messageA ...interface{}) {
	gitHubSourceCondSet.Manage(s).MarkFalse(GitHubSourceConditionSecretsProvided, reason, messageFormat, messageA...)
}

// MarkSink sets the condition that the source has a sink configured.
func (s *GitHubSourceStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		gitHubSourceCondSet.Manage(s).MarkTrue(GitHubSourceConditionSinkProvided)
	} else {
		gitHubSourceCondSet.Manage(s).MarkUnknown(GitHubSourceConditionSinkProvided,
			"SinkEmpty", "Sink has resolved to empty.")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *GitHubSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	gitHubSourceCondSet.Manage(s).MarkFalse(GitHubSourceConditionSinkProvided, reason, messageFormat, messageA...)
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GitHubSource is the Schema for the githubsources API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:categories=all,knative,eventing,sources
type GitHubSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitHubSourceSpec   `json:"spec,omitempty"`
	Status GitHubSourceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GitHubSourceList contains a list of GitHubSource
type GitHubSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitHubSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GitHubSource{}, &GitHubSourceList{})
}
