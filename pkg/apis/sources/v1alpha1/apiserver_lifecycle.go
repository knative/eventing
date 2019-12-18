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
	// ApiServerConditionReady has status True when the ApiServerSource is ready to send events.
	ApiServerConditionReady = apis.ConditionReady

	// ApiServerConditionSinkProvided has status True when the ApiServerSource has been configured with a sink target.
	ApiServerConditionSinkProvided apis.ConditionType = "SinkProvided"

	// ApiServerConditionDeployed has status True when the ApiServerSource has had it's deployment created.
	ApiServerConditionDeployed apis.ConditionType = "Deployed"

	// ApiServerConditionSufficientPermissions has status True when the ApiServerSource has sufficient permissions to access resources.
	ApiServerConditionSufficientPermissions apis.ConditionType = "SufficientPermissions"

	// ApiServerConditionEventTypeProvided has status True when the ApiServerSource has been configured with its event types.
	ApiServerConditionEventTypeProvided apis.ConditionType = "EventTypesProvided"
)

var apiserverCondSet = apis.NewLivingConditionSet(
	ApiServerConditionSinkProvided,
	ApiServerConditionDeployed,
	ApiServerConditionSufficientPermissions,
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *ApiServerSourceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
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

// MarkSinkWarnDeprecated sets the condition that the source has a sink configured and warns ref is deprecated.
func (s *ApiServerSourceStatus) MarkSinkWarnRefDeprecated(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		c := apis.Condition{
			Type:     ApiServerConditionSinkProvided,
			Status:   corev1.ConditionTrue,
			Severity: apis.ConditionSeverityError,
			Message:  "Using deprecated object ref fields when specifying spec.sink. Update to spec.sink.ref. These will be removed in the future.",
		}
		apiserverCondSet.Manage(s).SetCondition(c)
	} else {
		apiserverCondSet.Manage(s).MarkUnknown(ApiServerConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *ApiServerSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	apiserverCondSet.Manage(s).MarkFalse(ApiServerConditionSinkProvided, reason, messageFormat, messageA...)
}

// PropagateDeploymentAvailability uses the availability of the provided Deployment to determine if
// ApiServerConditionDeployed should be marked as true or false.
func (s *ApiServerSourceStatus) PropagateDeploymentAvailability(d *appsv1.Deployment) {
	if duck.DeploymentIsAvailable(&d.Status, false) {
		apiserverCondSet.Manage(s).MarkTrue(ApiServerConditionDeployed)
	} else {
		// I don't know how to propagate the status well, so just give the name of the Deployment
		// for now.
		apiserverCondSet.Manage(s).MarkFalse(ApiServerConditionDeployed, "DeploymentUnavailable", "The Deployment '%s' is unavailable.", d.Name)
	}
}

// MarkEventTypes sets the condition that the source has set its event type.
func (s *ApiServerSourceStatus) MarkEventTypes() {
	apiserverCondSet.Manage(s).MarkTrue(ApiServerConditionEventTypeProvided)
}

// MarkNoEventTypes sets the condition that the source does not its event type configured.
func (s *ApiServerSourceStatus) MarkNoEventTypes(reason, messageFormat string, messageA ...interface{}) {
	apiserverCondSet.Manage(s).MarkFalse(ApiServerConditionEventTypeProvided, reason, messageFormat, messageA...)
}

// MarkSufficientPermissions sets the condition that the source has enough permissions to access the resources.
func (s *ApiServerSourceStatus) MarkSufficientPermissions() {
	apiserverCondSet.Manage(s).MarkTrue(ApiServerConditionSufficientPermissions)
}

// MarkNoSufficientPermissions sets the condition that the source does not have enough permissions to access the resources
func (s *ApiServerSourceStatus) MarkNoSufficientPermissions(reason, messageFormat string, messageA ...interface{}) {
	apiserverCondSet.Manage(s).MarkFalse(ApiServerConditionSufficientPermissions, reason, messageFormat, messageA...)
}

// IsReady returns true if the resource is ready overall.
func (s *ApiServerSourceStatus) IsReady() bool {
	return apiserverCondSet.Manage(s).IsHappy()
}
