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

package v1

import (
	discoveryv1 "k8s.io/api/discovery/v1"

	"knative.dev/eventing/pkg/apis/duck"
	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
)

func (bs *BrokerStatus) MarkIngressFailed(reason, format string, args ...interface{}) {
	bs.GetConditionSet().Manage(bs).MarkFalse(BrokerConditionIngress, reason, format, args...)
}

func (bs *BrokerStatus) PropagateIngressAvailability(epSlices []*discoveryv1.EndpointSlice) {
	if duck.EndpointSlicesAreAvailable(epSlices) {
		bs.GetConditionSet().Manage(bs).MarkTrue(BrokerConditionIngress)
	} else {
		bs.MarkIngressFailed("EndpointSlicesUnavailable", "EndpointSlices are unavailable.")
	}
}

func (bs *BrokerStatus) MarkTriggerChannelFailed(reason, format string, args ...interface{}) {
	bs.GetConditionSet().Manage(bs).MarkFalse(BrokerConditionTriggerChannel, reason, format, args...)
}

func (bs *BrokerStatus) PropagateTriggerChannelReadiness(cs *duckv1.ChannelableStatus) {
	// TODO: Once you can get a Ready status from Channelable in a generic way, use it here...
	address := cs.AddressStatus.Address
	if address != nil {
		bs.GetConditionSet().Manage(bs).MarkTrue(BrokerConditionTriggerChannel)
	} else {
		bs.MarkTriggerChannelFailed("ChannelNotReady", "trigger Channel is not ready: not addressable")
	}
}

func (bs *BrokerStatus) MarkBrokerAddressableUnknown(reason, format string, args ...interface{}) {
	bs.GetConditionSet().Manage(bs).MarkUnknown(BrokerConditionAddressable, reason, format, args...)
}

func (bs *BrokerStatus) MarkFilterFailed(reason, format string, args ...interface{}) {
	bs.GetConditionSet().Manage(bs).MarkFalse(BrokerConditionFilter, reason, format, args...)
}

func (bs *BrokerStatus) PropagateFilterAvailability(epSlices []*discoveryv1.EndpointSlice) {
	if duck.EndpointSlicesAreAvailable(epSlices) {
		bs.GetConditionSet().Manage(bs).MarkTrue(BrokerConditionFilter)
	} else {
		bs.MarkFilterFailed("EndpointSlicesUnavailable", "EndpointSlices are unavailable.")
	}
}
