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
	// ApiServerConditionReady has status True when the ApiServerSource is ready to send events.
	ApiServerConditionReady = duckv1alpha1.ConditionReady

	// ApiServerConditionSinkProvided has status True when the ApiServerSource has been configured with a sink target.
	ApiServerConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"

	// ApiServerConditionDeployed has status True when the ApiServerSource has had it's deployment created.
	ApiServerConditionDeployed duckv1alpha1.ConditionType = "Deployed"
)

var apiserverCondSet = duckv1alpha1.NewLivingConditionSet(
	ApiServerConditionSinkProvided,
	ApiServerConditionDeployed,
)

// GetConditions returns Conditions
func (s *ApiServerSourceStatus) GetConditions() duckv1alpha1.Conditions {
	return s.Conditions
}

// SetConditions sets Conditions
func (s *ApiServerSourceStatus) SetConditions(conditions duckv1alpha1.Conditions) {
	s.Conditions = conditions
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *ApiServerSourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return apiserverCondSet.Manage(s).GetCondition(t)
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *ApiServerSourceStatus) InitializeConditions() {
	apiserverCondSet.Manage(s).InitializeConditions()
}

// MarkSink sets the condition that the source has a sink configured.
func (s *ApiServerSourceStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		apiserverCondSet.Manage(s).MarkTrue(ApiServerConditionSinkProvided)
	} else {
		apiserverCondSet.Manage(s).MarkUnknown(ApiServerConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *ApiServerSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	apiserverCondSet.Manage(s).MarkFalse(ApiServerConditionSinkProvided, reason, messageFormat, messageA...)
}

// MarkDeployed sets the condition that the source has been deployed.
func (s *ApiServerSourceStatus) MarkDeployed() {
	apiserverCondSet.Manage(s).MarkTrue(ApiServerConditionDeployed)
}

// IsReady returns true if the resource is ready overall.
func (s *ApiServerSourceStatus) IsReady() bool {
	return apiserverCondSet.Manage(s).IsHappy()
}
