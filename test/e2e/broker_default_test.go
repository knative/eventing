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
	"sort"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/uuid"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	pkgResources "knative.dev/eventing/pkg/reconciler/namespace/resources"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"
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
	// Be careful with the length of extension name and values,
	// we use extension name and value as a part of the name of resources like subscriber and trigger, the maximum characters allowed of resource name is 63
	extensionName1            = "extname1"
	extensionValue1           = "extval1"
	extensionName2            = "extname2"
	extensionValue2           = "extvalue2"
	nonMatchingExtensionName  = "nonmatchingextname"
	nonMatchingExtensionValue = "nonmatchingextval"
)

type eventContext struct {
	Type       string
	Source     string
	Extensions map[string]interface{}
}

// Helper struct to tie the type and sources of the events we expect to receive
// in subscribers with the selectors we use when creating their pods.
type eventReceiver struct {
	context  eventContext
	selector map[string]string
}

// This test annotates the testing namespace so that a default broker is created.
// It then binds many triggers with different filtering patterns to that default broker,
// and sends different events to the broker's address. Finally, it verifies that only
// the appropriate events are routed to the subscribers.
func TestDefaultBrokerWithManyTriggers(t *testing.T) {
	tests := []struct {
		name            string
		eventsToReceive []eventReceiver // These are the event context attributes and extension attributes that triggers will listen to,
		// to set in the subscriber and services pod
		eventsToSend            []eventContext // These are the event context attributes and extension attributes that will be send.
		deprecatedTriggerFilter bool           //TriggerFilter with DeprecatedSourceAndType or not
	}{
		{
			name: "test default broker with many deprecated triggers",
			eventsToReceive: []eventReceiver{
				{eventContext{Type: any, Source: any}, newSelector()},
				{eventContext{Type: eventType1, Source: any}, newSelector()},
				{eventContext{Type: any, Source: eventSource1}, newSelector()},
				{eventContext{Type: eventType1, Source: eventSource1}, newSelector()},
			},
			eventsToSend: []eventContext{
				{Type: eventType1, Source: eventSource1},
				{Type: eventType1, Source: eventSource2},
				{Type: eventType2, Source: eventSource1},
				{Type: eventType2, Source: eventSource2},
			},
			deprecatedTriggerFilter: true,
		}, {
			name: "test default broker with many attribute triggers",
			eventsToReceive: []eventReceiver{
				{eventContext{Type: any, Source: any}, newSelector()},
				{eventContext{Type: eventType1, Source: any}, newSelector()},
				{eventContext{Type: any, Source: eventSource1}, newSelector()},
				{eventContext{Type: eventType1, Source: eventSource1}, newSelector()},
			},
			eventsToSend: []eventContext{
				{Type: eventType1, Source: eventSource1},
				{Type: eventType1, Source: eventSource2},
				{Type: eventType2, Source: eventSource1},
				{Type: eventType2, Source: eventSource2},
			},
			deprecatedTriggerFilter: false,
		},
		{
			name: "test default broker with many attribute and extension triggers",
			eventsToReceive: []eventReceiver{
				{eventContext{Type: any, Source: any, Extensions: map[string]interface{}{extensionName1: extensionValue1}}, newSelector()},
				{eventContext{Type: any, Source: any, Extensions: map[string]interface{}{extensionName1: extensionValue1, extensionName2: extensionValue2}}, newSelector()},
				{eventContext{Type: any, Source: any, Extensions: map[string]interface{}{extensionName2: extensionValue2}}, newSelector()},
				{eventContext{Type: eventType1, Source: any, Extensions: map[string]interface{}{extensionName1: extensionValue1}}, newSelector()},
				{eventContext{Type: any, Source: any, Extensions: map[string]interface{}{extensionName1: any}}, newSelector()},
				{eventContext{Type: any, Source: eventSource1, Extensions: map[string]interface{}{extensionName1: extensionValue1}}, newSelector()},
				{eventContext{Type: any, Source: eventSource1, Extensions: map[string]interface{}{extensionName1: extensionValue1, extensionName2: extensionValue2}}, newSelector()},
			},
			eventsToSend: []eventContext{
				{Type: eventType1, Source: eventSource1, Extensions: map[string]interface{}{extensionName1: extensionValue1}},
				{Type: eventType1, Source: eventSource1, Extensions: map[string]interface{}{extensionName1: extensionValue1, extensionName2: extensionValue2}},
				{Type: eventType1, Source: eventSource1, Extensions: map[string]interface{}{extensionName2: extensionValue2}},
				{Type: eventType1, Source: eventSource2, Extensions: map[string]interface{}{extensionName1: extensionValue1}},
				{Type: eventType2, Source: eventSource1, Extensions: map[string]interface{}{extensionName1: nonMatchingExtensionValue}},
				{Type: eventType2, Source: eventSource2, Extensions: map[string]interface{}{nonMatchingExtensionName: extensionValue1}},
				{Type: eventType2, Source: eventSource2, Extensions: map[string]interface{}{extensionName1: extensionValue1, extensionName2: extensionValue2}},
				{Type: eventType2, Source: eventSource2, Extensions: map[string]interface{}{extensionName1: extensionValue1, nonMatchingExtensionName: extensionValue2}},
			},
			deprecatedTriggerFilter: false,
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
				subscriberName := name("dumper", event.context.Type, event.context.Source, event.context.Extensions)
				pod := resources.EventLoggerPod(subscriberName)
				client.CreatePodOrFail(pod, common.WithService(subscriberName))
			}

			// Create triggers.
			for _, event := range test.eventsToReceive {
				triggerName := name("trigger", event.context.Type, event.context.Source, event.context.Extensions)
				subscriberName := name("dumper", event.context.Type, event.context.Source, event.context.Extensions)
				triggerOption := getTriggerFilterOption(test.deprecatedTriggerFilter, event.context)
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
				// Using event type, source and extensions as part of the body for easier debugging.
				extensionsStr := joinSortedExtensions(eventToSend.Extensions)
				body := fmt.Sprintf(("Body-%s-%s-%s"), eventToSend.Type, eventToSend.Source, extensionsStr)
				cloudEvent := makeCloudEvent(eventToSend, body)
				// Create sender pod.
				senderPodName := name("sender", eventToSend.Type, eventToSend.Source, eventToSend.Extensions)
				if err := client.SendFakeEventToAddressable(senderPodName, defaultBrokerName, common.BrokerTypeMeta, cloudEvent); err != nil {
					t.Fatalf("Error send cloud event to broker: %v", err)
				}

				// Check on every dumper whether we should expect this event or not, and add its body
				// to the expectedEvents/unexpectedEvents maps.
				for _, eventToReceive := range test.eventsToReceive {
					subscriberName := name("dumper", eventToReceive.context.Type, eventToReceive.context.Source, eventToReceive.context.Extensions)
					if shouldExpectEvent(&eventToSend, &eventToReceive, t.Logf) {
						expectedEvents[subscriberName] = append(expectedEvents[subscriberName], body)
					} else {
						unexpectedEvents[subscriberName] = append(unexpectedEvents[subscriberName], body)
					}
				}
			}

			for _, event := range test.eventsToReceive {
				subscriberName := name("dumper", event.context.Type, event.context.Source, event.context.Extensions)
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

func makeCloudEvent(eventToSend eventContext, body string) *resources.CloudEvent {
	return &resources.CloudEvent{
		Source:     eventToSend.Source,
		Type:       eventToSend.Type,
		Extensions: eventToSend.Extensions,
		Data:       fmt.Sprintf(`{"msg":%q}`, body)}
}

func getTriggerFilterOption(deprecatedTriggerFilter bool, context eventContext) resources.TriggerOption {
	if deprecatedTriggerFilter {
		return resources.WithDeprecatedSourceAndTypeTriggerFilter(context.Source, context.Type)
	} else {
		return resources.WithAttributesTriggerFilter(context.Source, context.Type, context.Extensions)
	}
}

// Helper function to create names for different objects (e.g., triggers, services, etc.).
func name(obj, eventType, eventSource string, extensions map[string]interface{}) string {
	// Pod names need to be lowercase. We might have an eventType as Any, that is why we lowercase them.
	if eventType == "" {
		eventType = "testany"
	}
	if eventSource == "" {
		eventSource = "testany"
	}
	name := strings.ToLower(fmt.Sprintf("%s-%s-%s", obj, eventType, eventSource))
	if len(extensions) > 0 {
		name = strings.ToLower(fmt.Sprintf("%s-%s", name, joinSortedExtensions(extensions)))
	}
	return name
}

func joinSortedExtensions(extensions map[string]interface{}) string {
	var sb strings.Builder
	sortedExtensionNames := sortedKeys(extensions)
	for _, sortedExtensionName := range sortedExtensionNames {
		sb.WriteString("-")
		sb.WriteString(sortedExtensionName)
		sb.WriteString("-")
		vStr := fmt.Sprintf("%v", extensions[sortedExtensionName])
		if vStr == "" {
			vStr = "testany"
		}
		sb.WriteString(vStr)
	}
	return sb.String()
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Returns a new selector with a random uuid.
func newSelector() map[string]string {
	return map[string]string{selectorKey: string(uuid.NewUUID())}
}

// Checks whether we should expect to receive 'eventToSend' in 'eventReceiver' based on its type and source pattern.
func shouldExpectEvent(eventToSend *eventContext, receiver *eventReceiver, logf logging.FormatLogger) bool {
	if receiver.context.Type != any && receiver.context.Type != eventToSend.Type {
		return false
	}
	if receiver.context.Source != any && receiver.context.Source != eventToSend.Source {
		return false
	}
	for k, v := range receiver.context.Extensions {
		var value interface{}
		value, ok := eventToSend.Extensions[k]
		// If the attribute does not exist in the event, return false.
		if !ok {
			return false
		}
		// If the attribute is not set to any and is different than the one from the event, return false.
		if v != any && v != value {
			return false
		}
	}
	return true
}
