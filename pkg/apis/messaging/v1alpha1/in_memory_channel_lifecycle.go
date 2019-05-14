/*
 * Copyright 2019 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1alpha1

import (
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var imcCondSet = duckv1alpha1.NewLivingConditionSet(InMemoryChannelConditionDispatcherReady, InMemoryChannelConditionServiceReady, InMemoryChannelConditionEndpointsReady, InMemoryChannelConditionAddressable, InMemoryChannelConditionChannelServiceReady)

const (
	// InMemoryChannelConditionReady has status True when all subconditions below have been set to True.
	InMemoryChannelConditionReady = duckv1alpha1.ConditionReady

	// InMemoryChannelConditionDispatcherReady has status True when a Dispatcher deployment is ready
	// Keyed off appsv1.DeploymentAvaialble, which means minimum available replicas required are up
	// and running for at least minReadySeconds.
	InMemoryChannelConditionDispatcherReady duckv1alpha1.ConditionType = "DispatcherReady"

	// InMemoryChannelConditionServiceReady has status True when a k8s Service is ready. This
	// basically just means it exists because there's no meaningful status in Service. See Endpoints
	// below.
	InMemoryChannelConditionServiceReady duckv1alpha1.ConditionType = "ServiceReady"

	// InMemoryChannelConditionEndpointsReady has status True when a k8s Service Endpoints are backed
	// by at least one endpoint.
	InMemoryChannelConditionEndpointsReady duckv1alpha1.ConditionType = "EndpointsReady"

	// InMemoryChannelConditionAddressable has status true when this InMemoryChannel meets
	// the Addressable contract and has a non-empty hostname.
	InMemoryChannelConditionAddressable duckv1alpha1.ConditionType = "Addressable"

	// InMemoryChannelConditionServiceReady has status True when a k8s Service representing the channel is ready.
	// Because this uses ExternalName, there are no endpoints to check.
	InMemoryChannelConditionChannelServiceReady duckv1alpha1.ConditionType = "ChannelServiceReady"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (imcs *InMemoryChannelStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return imcCondSet.Manage(imcs).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (imcs *InMemoryChannelStatus) IsReady() bool {
	return imcCondSet.Manage(imcs).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (imcs *InMemoryChannelStatus) InitializeConditions() {
	imcCondSet.Manage(imcs).InitializeConditions()
}

// TODO: Use the new beta duck types.
func (imcs *InMemoryChannelStatus) SetAddress(hostname string) {
	imcs.Address.Hostname = hostname
	if hostname != "" {
		imcCondSet.Manage(imcs).MarkTrue(InMemoryChannelConditionAddressable)
	} else {
		imcCondSet.Manage(imcs).MarkFalse(InMemoryChannelConditionAddressable, "emptyHostname", "hostname is the empty string")
	}
}

func (imcs *InMemoryChannelStatus) MarkDispatcherFailed(reason, messageFormat string, messageA ...interface{}) {
	imcCondSet.Manage(imcs).MarkFalse(InMemoryChannelConditionDispatcherReady, reason, messageFormat, messageA...)
}

// TODO: Unify this with the ones from Eventing. Say: Broker, Trigger.
func (imcs *InMemoryChannelStatus) PropagateDispatcherStatus(ds *appsv1.DeploymentStatus) {
	for _, cond := range ds.Conditions {
		if cond.Type == appsv1.DeploymentAvailable {
			if cond.Status != corev1.ConditionTrue {
				imcs.MarkDispatcherFailed("DispatcherNotReady", "Dispatcher Deployment is not ready: %s : %s", cond.Reason, cond.Message)
			} else {
				imcCondSet.Manage(imcs).MarkTrue(InMemoryChannelConditionDispatcherReady)
			}
		}
	}
}

func (imcs *InMemoryChannelStatus) MarkServiceFailed(reason, messageFormat string, messageA ...interface{}) {
	imcCondSet.Manage(imcs).MarkFalse(InMemoryChannelConditionServiceReady, reason, messageFormat, messageA...)
}

func (imcs *InMemoryChannelStatus) MarkServiceTrue() {
	imcCondSet.Manage(imcs).MarkTrue(InMemoryChannelConditionServiceReady)
}

func (imcs *InMemoryChannelStatus) MarkChannelServiceFailed(reason, messageFormat string, messageA ...interface{}) {
	imcCondSet.Manage(imcs).MarkFalse(InMemoryChannelConditionChannelServiceReady, reason, messageFormat, messageA...)
}

func (imcs *InMemoryChannelStatus) MarkChannelServiceTrue() {
	imcCondSet.Manage(imcs).MarkTrue(InMemoryChannelConditionChannelServiceReady)
}

func (imcs *InMemoryChannelStatus) MarkEndpointsFailed(reason, messageFormat string, messageA ...interface{}) {
	imcCondSet.Manage(imcs).MarkFalse(InMemoryChannelConditionEndpointsReady, reason, messageFormat, messageA...)
}

func (imcs *InMemoryChannelStatus) MarkEndpointsTrue() {
	imcCondSet.Manage(imcs).MarkTrue(InMemoryChannelConditionEndpointsReady)
}
