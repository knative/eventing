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
	appsv1 "k8s.io/api/apps/v1"
	"knative.dev/eventing/pkg/apis/duck"
	"knative.dev/pkg/apis"
)

const (
	// IntegrationSourceConditionReady has status True when the IntegrationSource is ready to send events.
	IntegrationSourceConditionReady = apis.ConditionReady

	// IntegrationSourceConditionSinkProvided has status True when the ApiServerSource has been configured with a sink target.
	IntegrationSourceConditionSinkProvided apis.ConditionType = "SinkProvided"

	// IntegrationSourceConditionSinkBindingReady has status True when the IntegrationSource's SinkBinding is ready.
	IntegrationSourceConditionSinkBindingReady apis.ConditionType = "SinkBindingReady"

	// IntegrationSourceConditionReceiveAdapterReady has status True when the IntegrationSource's Deployment is ready.
	IntegrationSourceConditionDeploymentReady apis.ConditionType = "DeploymentReady"
)

var IntegrationCondSet = apis.NewLivingConditionSet(
	IntegrationSourceConditionSinkBindingReady,
	IntegrationSourceConditionDeploymentReady,
)

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*IntegrationSource) GetConditionSet() apis.ConditionSet {
	return IntegrationCondSet
}

func (iss *IntegrationSourceStatus) MarkSink(uri *apis.URL) {
	iss.SinkURI = uri
	if len(uri.String()) > 0 {
		IntegrationCondSet.Manage(iss).MarkTrue(IntegrationSourceConditionSinkProvided)
	} else {
		IntegrationCondSet.Manage(iss).MarkUnknown(IntegrationSourceConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.")
	}
}

func (iss *IntegrationSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	IntegrationCondSet.Manage(iss).MarkFalse(IntegrationSourceConditionSinkProvided, reason, messageFormat, messageA...)
}

func (iss *IntegrationSourceStatus) PropagateDeploymentAvailability(d *appsv1.Deployment) {
	if duck.DeploymentIsAvailable(&d.Status, false) {
		IntegrationCondSet.Manage(iss).MarkTrue(IntegrationSourceConditionDeploymentReady)
	} else {
		// I don't know how to propagate the status well, so just give the name of the Deployment
		// for now.
		IntegrationCondSet.Manage(iss).MarkFalse(IntegrationSourceConditionDeploymentReady, "DeploymentUnavailable", "The Deployment '%s' is unavailable.", d.Name)
	}
}

func (iss *IntegrationSourceStatus) IsReady() bool {
	return IntegrationCondSet.Manage(iss).IsHappy()
}
