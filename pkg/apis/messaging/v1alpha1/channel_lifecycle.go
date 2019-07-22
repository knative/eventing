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
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck/v1alpha1"
)

var chCondSet = apis.NewLivingConditionSet(ChannelConditionBackingChannelReady, ChannelConditionAddressable)

const (
	// ChannelConditionReady has status True when all subconditions below have been set to True.
	ChannelConditionReady = apis.ConditionReady

	// ChannelConditionAddressable has status true when this Channel meets
	// the Addressable contract and has a non-empty hostname.
	ChannelConditionAddressable apis.ConditionType = "Addressable"

	// ChannelConditionBackingChannelReady has status True when the backing Channel CRD is ready.
	ChannelConditionBackingChannelReady apis.ConditionType = "BackingChannelReady"
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

func (cs *ChannelStatus) MarkBackingChannelFailed(reason, messageFormat string, messageA ...interface{}) {
	chCondSet.Manage(cs).MarkFalse(ChannelConditionBackingChannelReady, reason, messageFormat, messageA...)
}

func (cs *ChannelStatus) PropagateChannelReadiness(chs *eventingduck.ChannelableStatus) {
	// TODO: Once you can get a Ready status from Channelable in a generic way, use it here...
	address := cs.AddressStatus.Address
	if address != nil {
		chCondSet.Manage(cs).MarkTrue(ChannelConditionBackingChannelReady)
	} else {
		cs.MarkBackingChannelFailed("ChannelNotReady", "Channel is not ready: not addressable")
	}
}
