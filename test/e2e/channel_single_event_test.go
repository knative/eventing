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

	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/e2e/helpers"
)

/*
SingleEventForChannelTestHelper tests the following scenario:

EventSource ---> Channel ---> Subscription ---> Service(Logger)

*/

func TestSingleBinaryEventForChannel(t *testing.T) {
	helpers.SingleEventForChannelTestHelper(
		t,
		resources.CloudEventEncodingBinary,
		channelTestRunner,
	)
}

func TestSingleStructuredEventForChannel(t *testing.T) {
	helpers.SingleEventForChannelTestHelper(
		t,
		resources.CloudEventEncodingStructured,
		channelTestRunner,
	)
}
