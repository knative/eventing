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

var imcCondSet = duckv1alpha1.NewLivingConditionSet(InMemoryChannelDispatcher, InMemoryChannelService, InMemoryChannelEndpoints)

const (
	// InMemoryChannelConditionReady has status True when all subconditions below have been set to True.
	InMemoryChannelConditionReady = duckv1alpha1.ConditionReady

	// InMemoryChannelDispatcher has status True when a Dispatcher deployment is ready
	InMemoryChannelDispatcher duckv1alpha1.ConditionType = "DispatcherReady"

	// InMemoryChannelService has status True when a k8s Service is ready
	InMemoryChannelService duckv1alpha1.ConditionType = "ServiceReady"

	// InMemoryChannelEndpoints has status True when a k8s Service Endpoints are backed
	// by at least one endpoint.
	InMemoryChannelEndpoints duckv1alpha1.ConditionType = "EndpointsReady"

	// ChannelConditionReady has status True when the Channel is ready to
	// accept traffic.
	ChannelConditionReady = duckv1alpha1.ConditionReady

	// ChannelConditionAddressable has status true when this Channel meets
	// the Addressable contract and has a non-empty hostname.
	ChannelConditionAddressable duckv1alpha1.ConditionType = "Addressable"
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

func (imcs *InMemoryChannelStatus) SetAddress(hostname string) {
	imcs.Address.Hostname = hostname
	if hostname != "" {
		imcCondSet.Manage(imcs).MarkTrue(ChannelConditionAddressable)
	} else {
		imcCondSet.Manage(imcs).MarkFalse(ChannelConditionAddressable, "emptyHostname", "hostname is the empty string")
	}
}

func (imcs *InMemoryChannelStatus) MarkDispatcherFailed(reason, messageFormat string, messageA ...interface{}) {
	imcCondSet.Manage(imcs).MarkFalse(InMemoryChannelDispatcher, reason, messageFormat, messageA...)
}

func (imcs *InMemoryChannelStatus) PropagateDispatcherStatus(ds *appsv1.DeploymentStatus) {
	for _, cond := range ds.Conditions {
		if cond.Type == appsv1.DeploymentAvailable {
			if cond.Status != corev1.ConditionTrue {
				imcs.MarkDispatcherFailed("DispatcherNotReady", "Dispatcher Deployment is not ready: %s : %s", cond.Reason, cond.Message)
			} else {
				imcCondSet.Manage(imcs).MarkTrue(InMemoryChannelDispatcher)
			}
		}
	}
}

func (imcs *InMemoryChannelStatus) MarkServiceFailed(reason, messageFormat string, messageA ...interface{}) {
	imcCondSet.Manage(imcs).MarkFalse(InMemoryChannelService, reason, messageFormat, messageA...)
}

func (imcs *InMemoryChannelStatus) MarkServiceTrue() {
	imcCondSet.Manage(imcs).MarkTrue(InMemoryChannelService)
}

func (imcs *InMemoryChannelStatus) MarkEndpointsFailed(reason, messageFormat string, messageA ...interface{}) {
	imcCondSet.Manage(imcs).MarkFalse(InMemoryChannelEndpoints, reason, messageFormat, messageA...)
}

func (imcs *InMemoryChannelStatus) MarkEndpointsTrue() {
	imcCondSet.Manage(imcs).MarkTrue(InMemoryChannelEndpoints)
}
