/*
Copyright 2026 The Knative Authors

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

package broker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBrokerSendEventWithOIDCTokenToReplyWaitsForHelperTrigger ensures the helper
// trigger, which routes the subscriber's reply to the reply sink, is ready before
// the event is sent, otherwise the reply is dropped.
func TestBrokerSendEventWithOIDCTokenToReplyWaitsForHelperTrigger(t *testing.T) {
	steps := BrokerSendEventWithOIDCTokenToReply().Steps

	helperTriggerReady, sendEvent := -1, -1
	for i, step := range steps {
		switch step.Name {
		case "helper trigger is ready":
			helperTriggerReady = i
		case "install source":
			sendEvent = i
		}
	}

	require.NotEqual(t, -1, helperTriggerReady, "step %q not found", "helper trigger is ready")
	require.NotEqual(t, -1, sendEvent, "step %q not found", "install source")
	require.Less(t, helperTriggerReady, sendEvent, "the helper trigger must be ready before the event is sent")
}
