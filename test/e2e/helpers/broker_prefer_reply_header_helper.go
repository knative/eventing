//go:build e2e
// +build e2e

/*
Copyright 2021 The Knative Authors

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
	"github.com/google/uuid"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
)

func BrokerPreferHeaderCheck(
	ctx context.Context,
	t *testing.T,
	creator BrokerCreator,
	options ...testlib.SetupClientOption) {
	const (
		recorderName = "event-recorder"
		triggerName  = "test-trigger"
		senderName   = "request-sender"
		eventSource  = "source1"
		eventType    = "type1"
		eventBody    = `{"msg":"test msg"}`
	)

	tests := []struct {
		name string
	}{
		{
			name: "test message without explicit prefer header should have the header",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := testlib.Setup(t, true, options...)
			defer testlib.TearDown(client)

			brokerName := creator(client, "v1")

			// Create event tracker that should receive all events.
			eventTracker, _ := recordevents.StartEventRecordOrFail(
				ctx,
				client,
				recorderName,
			)

			client.CreateTriggerOrFail(triggerName,
				resources.WithSubscriberServiceRefForTrigger(recorderName),
				resources.WithBroker(brokerName),
			)

			client.WaitForAllTestResourcesReadyOrFail(ctx)

			eventToSend := cloudevents.NewEvent()
			eventToSend.SetID(uuid.New().String())
			eventToSend.SetType(eventType)
			eventToSend.SetSource(eventSource)
			if err := eventToSend.SetData(cloudevents.ApplicationJSON, []byte(eventBody)); err != nil {
				t.Fatal("Cannot set the payload of the event:", err.Error())
			}
			client.SendEventToAddressable(
				ctx,
				senderName,
				brokerName,
				testlib.BrokerTypeMeta,
				eventToSend,
			)

			eventTracker.AssertAtLeast(
				1,
				recordevents.HasAdditionalHeader("Prefer", "reply"),
			)
		})
	}
}
