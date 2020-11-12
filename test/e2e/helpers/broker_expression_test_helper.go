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

package helpers

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cetest "github.com/cloudevents/sdk-go/v2/test"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
)

// TestBrokerWithExpressionTrigger tests broker filter using JS expressions
func TestBrokerWithExpressionTrigger(ctx context.Context, t *testing.T, brokerCreator BrokerCreator) {
	client := testlib.Setup(t, true)
	defer testlib.TearDown(client)

	brokerName := brokerCreator(client, "v1beta1")

	// Wait for broker ready.
	client.WaitForResourceReadyOrFail(brokerName, testlib.BrokerTypeMeta)

	subscriberNamePass := "recordevents-expression-pass"
	eventTrackerPass, _ := recordevents.StartEventRecordOrFail(ctx, client, subscriberNamePass)

	triggerNameOk := "trigger-expression-pass"
	client.CreateTriggerOrFailV1Beta1(triggerNameOk,
		resources.WithSubscriberServiceRefForTriggerV1Beta1(subscriberNamePass),
		resources.WithExpressionTriggerFilterV1Beta1("event.id != null"),
		resources.WithBrokerV1Beta1(brokerName),
	)

	subscriberNameFail := "recordevents-expression-fail"
	eventTrackerFail, _ := recordevents.StartEventRecordOrFail(ctx, client, subscriberNameFail)

	triggerNameFail := "trigger-expression-fail"
	client.CreateTriggerOrFailV1Beta1(triggerNameFail,
		resources.WithSubscriberServiceRefForTriggerV1Beta1(subscriberNameFail),
		resources.WithExpressionTriggerFilterV1Beta1("event.id == null"),
		resources.WithBrokerV1Beta1(brokerName),
	)

	// Wait for all test resources to become ready before sending the events.
	client.WaitForAllTestResourcesReadyOrFail(ctx)

	client.SendEventToAddressable(ctx, "sender", brokerName, testlib.BrokerTypeMeta, cetest.FullEvent())

	eventTrackerPass.AssertAtLeast(1, recordevents.MatchEvent(cetest.HasSpecVersion(cloudevents.VersionV1)))
	eventTrackerFail.AssertNot(recordevents.MatchEvent(cetest.HasSpecVersion(cloudevents.VersionV1)))
}
