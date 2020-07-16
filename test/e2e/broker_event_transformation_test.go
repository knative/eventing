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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/test/e2e/helpers"
	"knative.dev/eventing/test/lib"
)

/*
TestEventTransformationForTrigger tests the following scenario:

                         5                 4
                   ------------- ----------------------
                   |           | |                    |
             1     v	 2     | v        3           |
EventSource ---> Broker ---> Trigger1 -------> Service(Transformation)
                   |
                   | 6                   7
                   |-------> Trigger2 -------> Service(Logger)

Note: the number denotes the sequence of the event that flows in this test case.
*/
func TestEventTransformationForTriggerV1BrokerV1(t *testing.T) {
	runTest(t, "v1", "v1")
}

func TestEventTransformationForTriggerV1Beta1BrokerV1(t *testing.T) {
	runTest(t, "v1", "v1beta1")
}
func TestEventTransformationForTriggerV1Beta1BrokerV1Beta1(t *testing.T) {
	runTest(t, "v1beta1", "v1beta1")
}
func TestEventTransformationForTriggerV1BrokerV1Beta1(t *testing.T) {
	runTest(t, "v1beta1", "v1")
}

func runTest(t *testing.T, brokerVersion string, triggerVersion string) {

	channelTestRunner.RunTests(t, lib.FeatureBasic, func(t *testing.T, component metav1.TypeMeta) {
		helpers.EventTransformationForTriggerTestHelper(
			t,
			brokerVersion,
			triggerVersion,
			helpers.ChannelBasedBrokerCreator(component, brokerClass),
		)
	})

}
