// +build e2e

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

package e2e

import (
	"context"
	"testing"

	"knative.dev/eventing/test/e2e/helpers"
)

/*
TestBrokerChannelFlow tests the following topology:

                   ------------- ----------------------
                   |           | |                    |
                   v	       | v                    |
EventSource ---> Broker ---> Trigger1 -------> Service(Transformation)
                   |
                   |
                   |-------> Trigger2 -------> Service(Logger1)
                   |
                   |
                   |-------> Trigger3 -------> Channel --------> Subscription --------> Service(Logger2)

Explanation:
Trigger1 filters the orignal event and transforms it to a new event,
Trigger2 logs all events,
Trigger3 filters the transformed event and sends it to Channel.

*/
func TestBrokerChannelFlowTriggerV1BrokerV1(t *testing.T) {
	helpers.BrokerChannelFlowWithTransformation(context.Background(), t, brokerClass, "v1", "v1", channelTestRunner)
}
func TestBrokerChannelFlowV1Beta1BrokerV1(t *testing.T) {
	helpers.BrokerChannelFlowWithTransformation(context.Background(), t, brokerClass, "v1", "v1beta1", channelTestRunner)
}
func TestBrokerChannelFlowTriggerV1Beta1BrokerV1Beta1(t *testing.T) {
	helpers.BrokerChannelFlowWithTransformation(context.Background(), t, brokerClass, "v1beta1", "v1beta1", channelTestRunner)
}
func TestBrokerChannelFlowTriggerV1BrokerV1Beta1(t *testing.T) {
	helpers.BrokerChannelFlowWithTransformation(context.Background(), t, brokerClass, "v1beta1", "v1", channelTestRunner)
}
