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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck/v1alpha1"
)

var chCondSet = apis.NewLivingConditionSet(ChannelConditionDispatcherReady, ChannelConditionServiceReady, ChannelConditionEndpointsReady, ChannelConditionAddressable, ChannelConditionChannelServiceReady)

const (
	// ChannelConditionReady has status True when all subconditions below have been set to True.
	ChannelConditionReady = apis.ConditionReady

	// ChannelConditionDispatcherReady has status True when a Dispatcher deployment is ready
	// Keyed off appsv1.DeploymentAvailable, which means minimum available replicas required are up
	// and running for at least minReadySeconds.
	ChannelConditionDispatcherReady apis.ConditionType = "DispatcherReady"

	// ChannelConditionServiceReady has status True when a k8s Service is ready. This
	// basically just means it exists because there's no meaningful status in Service. See Endpoints
	// below.
	ChannelConditionServiceReady apis.ConditionType = "ServiceReady"

	// ChannelConditionEndpointsReady has status True when a k8s Service Endpoints are backed
	// by at least one endpoint.
	ChannelConditionEndpointsReady apis.ConditionType = "EndpointsReady"

	// ChannelConditionAddressable has status true when this Channel meets
	// the Addressable contract and has a non-empty hostname.
	ChannelConditionAddressable apis.ConditionType = "Addressable"

	// ChannelConditionServiceReady has status True when a k8s Service representing the channel is ready.
	// Because this uses ExternalName, there are no endpoints to check.
	ChannelConditionChannelServiceReady apis.ConditionType = "ChannelServiceReady"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (cs *ChannelStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return chCondSet.Manage(cs).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (cs *ChannelStatus) IsReady() bool {
	return chCondSet.Manage(cs).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (cs *ChannelStatus) InitializeConditions() {
	chCondSet.Manage(cs).InitializeConditions()
}

func (cs *ChannelStatus) SetAddress(url *apis.URL) {
	if cs.Address == nil {
		cs.Address = &v1alpha1.Addressable{}
	}
	if url != nil {
		cs.Address.Hostname = url.Host
		cs.Address.URL = url
		chCondSet.Manage(cs).MarkTrue(ChannelConditionAddressable)
	} else {
		cs.Address.Hostname = ""
		cs.Address.URL = nil
		chCondSet.Manage(cs).MarkFalse(ChannelConditionAddressable, "EmptyHostname", "hostname is the empty string")
	}
}

func (cs *ChannelStatus) MarkDispatcherFailed(reason, messageFormat string, messageA ...interface{}) {
	chCondSet.Manage(cs).MarkFalse(ChannelConditionDispatcherReady, reason, messageFormat, messageA...)
}

// TODO: Unify this with the ones from Eventing. Say: Broker, Trigger.
func (cs *ChannelStatus) PropagateDispatcherStatus(ds *appsv1.DeploymentStatus) {
	for _, cond := range ds.Conditions {
		if cond.Type == appsv1.DeploymentAvailable {
			if cond.Status != corev1.ConditionTrue {
				cs.MarkDispatcherFailed("DispatcherNotReady", "Dispatcher Deployment is not ready: %s : %s", cond.Reason, cond.Message)
			} else {
				chCondSet.Manage(cs).MarkTrue(ChannelConditionDispatcherReady)
			}
		}
	}
}

func (cs *ChannelStatus) MarkServiceFailed(reason, messageFormat string, messageA ...interface{}) {
	chCondSet.Manage(cs).MarkFalse(ChannelConditionServiceReady, reason, messageFormat, messageA...)
}

func (cs *ChannelStatus) MarkServiceTrue() {
	chCondSet.Manage(cs).MarkTrue(ChannelConditionServiceReady)
}

func (cs *ChannelStatus) MarkChannelServiceFailed(reason, messageFormat string, messageA ...interface{}) {
	chCondSet.Manage(cs).MarkFalse(ChannelConditionChannelServiceReady, reason, messageFormat, messageA...)
}

func (cs *ChannelStatus) MarkChannelServiceTrue() {
	chCondSet.Manage(cs).MarkTrue(ChannelConditionChannelServiceReady)
}

func (cs *ChannelStatus) MarkEndpointsFailed(reason, messageFormat string, messageA ...interface{}) {
	chCondSet.Manage(cs).MarkFalse(ChannelConditionEndpointsReady, reason, messageFormat, messageA...)
}

func (cs *ChannelStatus) MarkEndpointsTrue() {
	chCondSet.Manage(cs).MarkTrue(ChannelConditionEndpointsReady)
}
