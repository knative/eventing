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
	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	// IntegrationSinkConditionReady has status True when the IntegrationSink is ready to send events.
	IntegrationSinkConditionReady = apis.ConditionReady

	IntegrationSinkConditionAddressable apis.ConditionType = "Addressable"

	// IntegrationSinkConditionDeploymentReady has status True when the IntegrationSink has been configured with a deployment.
	IntegrationSinkConditionDeploymentReady apis.ConditionType = "DeploymentReady"

	// IntegrationSinkConditionEventPoliciesReady has status True when all the applying EventPolicies for this
	// IntegrationSink are ready.
	IntegrationSinkConditionEventPoliciesReady apis.ConditionType = "EventPoliciesReady"

	// IntegrationSinkConditionCertificateReady has status True when the IntegrationSink's certificate is ready.
	IntegrationSinkConditionCertificateReady apis.ConditionType = "CertificateReady"

	// Certificate related condition reasons
	IntegrationSinkCertificateNotReady string = "CertificateNotReady"
)

var IntegrationSinkCondSet = apis.NewLivingConditionSet(
	IntegrationSinkConditionAddressable,
	IntegrationSinkConditionDeploymentReady,
	IntegrationSinkConditionEventPoliciesReady,
)

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*IntegrationSink) GetConditionSet() apis.ConditionSet {
	return IntegrationSinkCondSet
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *IntegrationSinkStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return IntegrationSinkCondSet.Manage(s).GetCondition(t)
}

// GetTopLevelCondition returns the top level Condition.
func (ps *IntegrationSinkStatus) GetTopLevelCondition() *apis.Condition {
	return IntegrationSinkCondSet.Manage(ps).GetTopLevelCondition()
}

// IsReady returns true if the resource is ready overall.
func (s *IntegrationSinkStatus) IsReady() bool {
	return IntegrationSinkCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *IntegrationSinkStatus) InitializeConditions() {
	IntegrationSinkCondSet.Manage(s).InitializeConditions()
}

// MarkAddressableReady marks the Addressable condition to True.
func (s *IntegrationSinkStatus) MarkAddressableReady() {
	IntegrationSinkCondSet.Manage(s).MarkTrue(IntegrationSinkConditionAddressable)
}

// MarkEventPoliciesFailed marks the EventPoliciesReady condition to False with the given reason and message.
func (s *IntegrationSinkStatus) MarkEventPoliciesFailed(reason, messageFormat string, messageA ...interface{}) {
	IntegrationSinkCondSet.Manage(s).MarkFalse(IntegrationSinkConditionEventPoliciesReady, reason, messageFormat, messageA...)
}

// MarkEventPoliciesUnknown marks the EventPoliciesReady condition to Unknown with the given reason and message.
func (s *IntegrationSinkStatus) MarkEventPoliciesUnknown(reason, messageFormat string, messageA ...interface{}) {
	IntegrationSinkCondSet.Manage(s).MarkUnknown(IntegrationSinkConditionEventPoliciesReady, reason, messageFormat, messageA...)
}

// MarkEventPoliciesTrue marks the EventPoliciesReady condition to True.
func (s *IntegrationSinkStatus) MarkEventPoliciesTrue() {
	IntegrationSinkCondSet.Manage(s).MarkTrue(IntegrationSinkConditionEventPoliciesReady)
}

// MarkEventPoliciesTrueWithReason marks the EventPoliciesReady condition to True with the given reason and message.
func (s *IntegrationSinkStatus) MarkEventPoliciesTrueWithReason(reason, messageFormat string, messageA ...interface{}) {
	IntegrationSinkCondSet.Manage(s).MarkTrueWithReason(IntegrationSinkConditionEventPoliciesReady, reason, messageFormat, messageA...)
}

func (s *IntegrationSinkStatus) PropagateDeploymentStatus(d *appsv1.DeploymentStatus) {
	deploymentAvailableFound := false
	for _, cond := range d.Conditions {
		if cond.Type == appsv1.DeploymentAvailable {
			deploymentAvailableFound = true
			if cond.Status == corev1.ConditionTrue {
				IntegrationSinkCondSet.Manage(s).MarkTrue(IntegrationSinkConditionDeploymentReady)
			} else if cond.Status == corev1.ConditionFalse {
				IntegrationSinkCondSet.Manage(s).MarkFalse(IntegrationSinkConditionDeploymentReady, cond.Reason, cond.Message)
			} else if cond.Status == corev1.ConditionUnknown {
				IntegrationSinkCondSet.Manage(s).MarkUnknown(IntegrationSinkConditionDeploymentReady, cond.Reason, cond.Message)
			}
		}
	}
	if !deploymentAvailableFound {
		IntegrationSinkCondSet.Manage(s).MarkUnknown(IntegrationSinkConditionDeploymentReady, "DeploymentUnavailable", "The Deployment '%s' is unavailable.", d)
	}
}

func (s *IntegrationSinkStatus) PropagateCertificateStatus(cs cmv1.CertificateStatus) bool {
	var topLevel *cmv1.CertificateCondition
	for _, cond := range cs.Conditions {
		if cond.Type == cmv1.CertificateConditionReady {
			topLevel = &cond
			break
		}
	}

	if topLevel == nil {
		IntegrationSinkCondSet.Manage(s).MarkUnknown(IntegrationSinkConditionCertificateReady,
			IntegrationSinkCertificateNotReady, "Certificate is progressing")
		return false
	}

	if topLevel.Status == cmmeta.ConditionUnknown {
		IntegrationSinkCondSet.Manage(s).MarkUnknown(IntegrationSinkConditionCertificateReady,
			IntegrationSinkCertificateNotReady, "Certificate is progressing, "+topLevel.Reason+" Message: "+topLevel.Message)
		return false
	}

	if topLevel.Status == cmmeta.ConditionFalse {
		IntegrationSinkCondSet.Manage(s).MarkFalse(IntegrationSinkConditionCertificateReady,
			IntegrationSinkCertificateNotReady, "Certificate is not ready, "+topLevel.Reason+" Message: "+topLevel.Message)
		return false
	}

	IntegrationSinkCondSet.Manage(s).MarkTrue(IntegrationSinkConditionCertificateReady)
	return true
}

func (s *IntegrationSinkStatus) SetAddress(address *duckv1.Addressable) {
	s.Address = address
	if address == nil || address.URL.IsEmpty() {
		IntegrationSinkCondSet.Manage(s).MarkFalse(IntegrationSinkConditionAddressable, "EmptyHostname", "hostname is the empty string")
	} else {
		IntegrationSinkCondSet.Manage(s).MarkTrue(IntegrationSinkConditionAddressable)

	}
}
