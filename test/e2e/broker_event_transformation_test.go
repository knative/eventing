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

	"knative.dev/eventing/test/e2e/helpers"
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
func TestEventTransformationForTrigger(t *testing.T) {
	helpers.EventTransformationForTriggerTestHelper(t, channelTestRunner)
}
