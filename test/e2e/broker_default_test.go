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
	"fmt"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/uuid"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	pkgResources "knative.dev/eventing/pkg/reconciler/namespace/resources"
	"knative.dev/pkg/test/logging"
)

const (
	waitForFilterPodRunning = 30 * time.Second
	selectorKey             = "end2end-test-broker-trigger"

	defaultBrokerName = pkgResources.DefaultBrokerName
	any               = v1alpha1.TriggerAnyFilter
	eventType1        = "type1"
	eventType2        = "type2"
	eventSource1      = "source1"
	eventSource2      = "source2"
	nilString         = "nilstring"
	extensionName     = `myextname`
	extensionValue    = `myextval`
)

type eventMeta struct {
	Type           string
	Source         string
	ExtensionName  string
	ExtensionValue string
}

// Helper struct to tie the type and sources of the events we expect to receive
// in subscribers with the selectors we use when creating their pods.
type eventReceiver struct {
	meta     eventMeta
	selector map[string]string
}

// This test annotates the testing namespace so that a default broker is created.
// It then binds many triggers with different filtering patterns to that default broker,
// and sends different events to the broker's address. Finally, it verifies that only
// the appropriate events are routed to the subscribers.
func TestDefaultBrokerWithManyTriggers(t *testing.T) {
	tests := []struct {
		name            string
		eventsToReceive []eventReceiver // These are the event types and sources that triggers will listen to, as well as the selectors
		// to set  in the subscriber and services pods.
		eventsToSend            []eventMeta // These are the event types and sources that will be send.
		deprecatedTriggerFilter bool        //TriggerFilter with DeprecatedSourceAndType or not
		extensionExists         bool
	}{{
		name: "test default broker with many deprecated triggers",
		eventsToReceive: []eventReceiver{
			{eventMeta{Type: any, Source: any, ExtensionName: nilString, ExtensionValue: nilString}, newSelector()},
			{eventMeta{Type: eventType1, Source: any, ExtensionName: nilString, ExtensionValue: nilString}, newSelector()},
			{eventMeta{Type: any, Source: eventSource1, ExtensionName: nilString, ExtensionValue: nilString}, newSelector()},
			{eventMeta{Type: eventType1, Source: eventSource1, ExtensionName: nilString, ExtensionValue: nilString}, newSelector()},
		},
		eventsToSend: []eventMeta{
			{eventType1, eventSource1, nilString, nilString},
			{eventType1, eventSource2, nilString, nilString},
			{eventType2, eventSource1, nilString, nilString},
			{eventType2, eventSource2, nilString, nilString},
		},
		deprecatedTriggerFilter: true,
		extensionExists:         false,
	}, {
		name: "test default broker with many attribute triggers",
		eventsToReceive: []eventReceiver{
			{eventMeta{Type: any, Source: any, ExtensionName: nilString, ExtensionValue: nilString}, newSelector()},
			{eventMeta{Type: eventType1, Source: any, ExtensionName: nilString, ExtensionValue: nilString}, newSelector()},
			{eventMeta{Type: any, Source: eventSource1, ExtensionName: nilString, ExtensionValue: nilString}, newSelector()},
			{eventMeta{Type: eventType1, Source: eventSource1, ExtensionName: nilString, ExtensionValue: nilString}, newSelector()},
		},
		eventsToSend: []eventMeta{
			{eventType1, eventSource1, nilString, nilString},
			{eventType1, eventSource2, nilString, nilString},
			{eventType2, eventSource1, nilString, nilString},
			{eventType2, eventSource2, nilString, nilString},
		},
		deprecatedTriggerFilter: false,
		extensionExists:         false,
	},
		{
			name: "test default broker with many attribute and extension triggers",
			eventsToReceive: []eventReceiver{
				{eventMeta{Type: any, Source: any, ExtensionName: extensionName, ExtensionValue: extensionValue}, newSelector()},
				{eventMeta{Type: eventType1, Source: any, ExtensionName: extensionName, ExtensionValue: extensionValue}, newSelector()},
				{eventMeta{Type: any, Source: any, ExtensionName: extensionName, ExtensionValue: any}, newSelector()},
				{eventMeta{Type: any, Source: eventSource1, ExtensionName: extensionName, ExtensionValue: extensionValue}, newSelector()},
			},
			eventsToSend: []eventMeta{
				{eventType1, eventSource1, extensionName, extensionValue},
				{eventType1, eventSource2, extensionName, extensionValue},
				{eventType2, eventSource1, extensionName, "non.matching.ext.val"},
				{eventType2, eventSource2, "non.matching.ext.name", extensionValue},
			},
			deprecatedTriggerFilter: false,
			extensionExists:         true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := setup(t, true)
			defer tearDown(client)

			// Label namespace so that it creates the default broker.
			if err := client.LabelNamespace(map[string]string{"knative-eventing-injection": "enabled"}); err != nil {
				t.Fatalf("Error annotating namespace: %v", err)
			}

			// Wait for default broker ready.
			if err := client.WaitForResourceReady(defaultBrokerName, common.BrokerTypeMeta); err != nil {
				t.Fatalf("Error waiting for default broker to become ready: %v", err)
			}

			// Create subscribers.
			for _, event := range test.eventsToReceive {
				subscriberName := name("dumper", event.meta.Type, event.meta.Source, event.meta.ExtensionName, event.meta.ExtensionValue)
				pod := resources.EventLoggerPod(subscriberName)
				client.CreatePodOrFail(pod, common.WithService(subscriberName))
			}

			// Create triggers.
			for _, event := range test.eventsToReceive {
				triggerName := name("trigger", event.meta.Type, event.meta.Source, event.meta.ExtensionName, event.meta.ExtensionValue)
				subscriberName := name("dumper", event.meta.Type, event.meta.Source, event.meta.ExtensionName, event.meta.ExtensionValue)
				triggerOption := getTriggerFilterOption(test.deprecatedTriggerFilter, test.extensionExists, event.meta)
				client.CreateTriggerOrFail(triggerName,
					resources.WithSubscriberRefForTrigger(subscriberName),
					triggerOption,
				)
			}

			// Wait for all test resources to become ready before sending the events.
			if err := client.WaitForAllTestResourcesReady(); err != nil {
				t.Fatalf("Failed to get all test resources ready: %v", err)
			}

			// Map to save the expected events per dumper so that we can verify the delivery.
			expectedEvents := make(map[string][]string)
			// Map to save the unexpected events per dumper so that we can verify that they weren't delivered.
			unexpectedEvents := make(map[string][]string)
			for _, eventToSend := range test.eventsToSend {
				// Create cloud event.
				// Using event type and source as part of the body for easier debugging.
				body := fmt.Sprintf("Body:eventType[%s]-eventSource[%s]-eventExtensionName[%s]-eventExtensionValue[%s]",
					eventToSend.Type, eventToSend.Source, eventToSend.ExtensionName, eventToSend.ExtensionValue)
				cloudEvent := makeCloudEvent(eventToSend, body)
				// Create sender pod.
				senderPodName := name("sender", eventToSend.Type, eventToSend.Source, eventToSend.ExtensionName, eventToSend.ExtensionValue)
				if err := client.SendFakeEventToAddressable(senderPodName, defaultBrokerName, common.BrokerTypeMeta, cloudEvent); err != nil {
					t.Fatalf("Error send cloud event to broker: %v", err)
				}

				// Check on every dumper whether we should expect this event or not, and add its body
				// to the expectedEvents/unexpectedEvents maps.
				for _, eventToReceive := range test.eventsToReceive {
					subscriberName := name("dumper", eventToReceive.meta.Type, eventToReceive.meta.Source, eventToReceive.meta.ExtensionName, eventToReceive.meta.ExtensionValue)
					if shouldExpectEvent(&eventToSend, &eventToReceive, t.Logf) {
						expectedEvents[subscriberName] = append(expectedEvents[subscriberName], body)
					} else {
						unexpectedEvents[subscriberName] = append(unexpectedEvents[subscriberName], body)
					}
				}
			}

			for _, event := range test.eventsToReceive {
				subscriberName := name("dumper", event.meta.Type, event.meta.Source, event.meta.ExtensionName, event.meta.ExtensionValue)
				if err := client.CheckLog(subscriberName, common.CheckerContainsAll(expectedEvents[subscriberName])); err != nil {
					t.Fatalf("Event(s) not found in logs of subscriber pod %q: %v", subscriberName, err)
				}
				// At this point all the events should have been received in the pod.
				// We check whether we find unexpected events. If so, then we fail.
				found, err := client.FindAnyLogContents(subscriberName, unexpectedEvents[subscriberName])
				if err != nil {
					t.Fatalf("Failed querying to find log contents in pod %q: %v", subscriberName, err)
				}
				if found {
					t.Fatalf("Unexpected event(s) found in logs of subscriber pod %q", subscriberName)
				}
			}
		})
	}

}

func makeCloudEvent(eventToSend eventMeta, body string) *resources.CloudEvent {
	if eventToSend.ExtensionName != nilString {
		return &resources.CloudEvent{
			Source:         eventToSend.Source,
			Type:           eventToSend.Type,
			ExtensionName:  eventToSend.ExtensionName,
			ExtensionValue: eventToSend.ExtensionValue,
			Data:           fmt.Sprintf(`{"msg":%q}`, body),
		}
	} else {
		return &resources.CloudEvent{
			Source: eventToSend.Source,
			Type:   eventToSend.Type,
			Data:   fmt.Sprintf(`{"msg":%q}`, body),
		}
	}
}

func getTriggerFilterOption(deprecatedTriggerFilter, extensionExists bool, eventMeta eventMeta) resources.TriggerOption {
	if deprecatedTriggerFilter {
		return resources.WithDeprecatedSourceAndTypeTriggerFilter(eventMeta.Source, eventMeta.Type)
	} else {
		if extensionExists {
			return resources.WithAttributesAndExtensionTriggerFilter(eventMeta.Source, eventMeta.Type, eventMeta.ExtensionName, eventMeta.ExtensionValue)
		} else {
			return resources.WithAttributesTriggerFilter(eventMeta.Source, eventMeta.Type)
		}
	}
}

// Helper function to create names for different objects (e.g., triggers, services, etc.).
func name(obj, eventType, eventSource, eventExtensionName, eventExtensionValue string) string {
	// Pod names need to be lowercase. We might have an eventType as Any, that is why we lowercase them.
	if eventType == "" {
		eventType = "testany"
	}
	if eventSource == "" {
		eventSource = "testany"
	}
	if eventExtensionName == nilString {
		eventExtensionName = "notexists"
	}
	if eventExtensionValue == "" {
		eventExtensionValue = "testany"
	}
	if eventExtensionValue == nilString {
		eventExtensionValue = "notexists"
	}
	return strings.ToLower(fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		obj,
		eventType,
		eventSource,
		eventExtensionName,
		eventExtensionValue))
}

// Returns a new selector with a random uuid.
func newSelector() map[string]string {
	return map[string]string{selectorKey: string(uuid.NewUUID())}
}

// Checks whether we should expect to receive 'eventToSend' in 'eventReceiver' based on its type and source pattern.
func shouldExpectEvent(eventToSend *eventMeta, receiver *eventReceiver, logf logging.FormatLogger) bool {
	if receiver.meta.Type != any && receiver.meta.Type != eventToSend.Type {
		return false
	}
	if receiver.meta.Source != any && receiver.meta.Source != eventToSend.Source {
		return false
	}
	//event extension does not exists, return True
	if receiver.meta.ExtensionName == nilString {
		return true
	}
	if receiver.meta.ExtensionName != eventToSend.ExtensionName {
		return false
	}
	if receiver.meta.ExtensionValue != any && receiver.meta.ExtensionValue != eventToSend.ExtensionValue {
		return false
	}
	return true
}
