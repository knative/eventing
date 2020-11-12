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

package upgrade

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"knative.dev/eventing/test/e2e/helpers"
	"knative.dev/eventing/test/lib"
)

var channelTestRunner lib.ComponentsTestRunner

func runSmokeTest(t *testing.T) {
	helpers.SingleEventForChannelTestHelper(
		context.Background(),
		t,
		cloudevents.EncodingBinary,
		helpers.SubscriptionV1beta1,
		"",
		channelTestRunner,
	)
}
