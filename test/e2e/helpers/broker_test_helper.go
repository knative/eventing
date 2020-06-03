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
	"fmt"
	"sort"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/cloudevents"
	"knative.dev/eventing/test/lib/resources"
)

const (
	any          = v1beta1.TriggerAnyFilter
	eventType1   = "type1"
	eventType2   = "type2"
	eventSource1 = "source1"
	eventSource2 = "source2"
	// Be careful with the length of extension name and values,
	// we use extension name and value as a part of the name of resources like subscriber and trigger,
	// the maximum characters allowed of resource name is 63
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
	context eventContext
}

// BrokerCreator creates a broker and returns its broker name.
// TestBrokerWithManyTriggers will wait for the broker to become ready.
type BrokerCreator func(client *lib.Client) string

// ChannelBasedBrokerCreator creates a BrokerCreator that creates a broker based on the channel parameter.
func ChannelBasedBrokerCreator(channel metav1.TypeMeta, brokerClass string) BrokerCreator {
	return func(client *lib.Client) string {
		brokerName := strings.ToLower(channel.Kind)

		// create a ConfigMap used by the broker.
		config := client.CreateBrokerConfigMapOrFail("config-"+brokerName, &channel)

		// create a new broker.
		client.CreateBrokerV1Beta1OrFail(brokerName,
			resources.WithBrokerClassForBrokerV1Beta1(brokerClass),
			resources.WithConfigForBrokerV1Beta1(config),
		)

		return brokerName
	}
}

// If shouldLabelNamespace is set to true this test annotates the testing namespace so that a default broker is created.
// It then binds many triggers with different filtering patterns to the broker created by brokerCreator, and sends
// different events to the broker's address.
// Finally, it verifies that only the appropriate events are routed to the subscribers.
func TestBrokerWithManyTriggers(t *testing.T, brokerCreator BrokerCreator, shouldLabelNamespace bool) {
	tests := []struct {
		name string
		// These are the event context attributes and extension attributes that triggers will listen to,
		// to set in the subscriber and services pod
		eventsToReceive []eventReceiver
		// These are the event context attributes and extension attributes that will be send.
		eventsToSend []eventContext
		//TriggerFilter with DeprecatedSourceAndType or not
		deprecatedTriggerFilter bool
		// Use v1beta1 trigger
		v1beta1 bool
	}{
		{
			name: "test default broker with many deprecated triggers",
			eventsToReceive: []eventReceiver{
				{eventContext{Type: any, Source: any}},
				{eventContext{Type: eventType1, Source: any}},
				{eventContext{Type: any, Source: eventSource1}},
				{eventContext{Type: eventType1, Source: eventSource1}},
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
				{eventContext{Type: any, Source: any}},
				{eventContext{Type: eventType1, Source: any}},
				{eventContext{Type: any, Source: eventSource1}},
				{eventContext{Type: eventType1, Source: eventSource1}},
			},
			eventsToSend: []eventContext{
				{Type: eventType1, Source: eventSource1},
				{Type: eventType1, Source: eventSource2},
				{Type: eventType2, Source: eventSource1},
				{Type: eventType2, Source: eventSource2},
			},
			deprecatedTriggerFilter: false,
		}, {
			name: "test default broker with many attribute triggers using v1beta1 trigger",
			eventsToReceive: []eventReceiver{
				{eventContext{Type: any, Source: any}},
				{eventContext{Type: eventType1, Source: any}},
				{eventContext{Type: any, Source: eventSource1}},
				{eventContext{Type: eventType1, Source: eventSource1}},
			},
			eventsToSend: []eventContext{
				{Type: eventType1, Source: eventSource1},
				{Type: eventType1, Source: eventSource2},
				{Type: eventType2, Source: eventSource1},
				{Type: eventType2, Source: eventSource2},
			},
			deprecatedTriggerFilter: false,
			v1beta1:                 true,
		}, {
			name: "test default broker with many attribute and extension triggers",
			eventsToReceive: []eventReceiver{
				{eventContext{Type: any, Source: any, Extensions: map[string]interface{}{extensionName1: extensionValue1}}},
				{eventContext{Type: any, Source: any, Extensions: map[string]interface{}{extensionName1: extensionValue1, extensionName2: extensionValue2}}},
				{eventContext{Type: any, Source: any, Extensions: map[string]interface{}{extensionName2: extensionValue2}}},
				{eventContext{Type: eventType1, Source: any, Extensions: map[string]interface{}{extensionName1: extensionValue1}}},
				{eventContext{Type: any, Source: any, Extensions: map[string]interface{}{extensionName1: any}}},
				{eventContext{Type: any, Source: eventSource1, Extensions: map[string]interface{}{extensionName1: extensionValue1}}},
				{eventContext{Type: any, Source: eventSource1, Extensions: map[string]interface{}{extensionName1: extensionValue1, extensionName2: extensionValue2}}},
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
			client := lib.Setup(t, true)
			defer lib.TearDown(client)

			if shouldLabelNamespace {
				// Label namespace so that it creates the default broker.
				if err := client.LabelNamespace(map[string]string{"knative-eventing-injection": "enabled"}); err != nil {
					t.Fatalf("Error annotating namespace: %v", err)
				}
			}

			brokerName := brokerCreator(client)

			// Wait for broker ready.
			client.WaitForResourceReadyOrFail(brokerName, lib.BrokerTypeMeta)

			if shouldLabelNamespace {
				// Test if namespace reconciler would recreate broker once broker was deleted.
				if err := client.Eventing.EventingV1beta1().Brokers(client.Namespace).Delete(brokerName, &metav1.DeleteOptions{}); err != nil {
					t.Fatalf("Can't delete default broker in namespace: %v", client.Namespace)
				}
				client.WaitForResourceReadyOrFail(brokerName, lib.BrokerTypeMeta)
			}

			// Create subscribers.
			for _, event := range test.eventsToReceive {
				subscriberName := name("dumper", event.context.Type, event.context.Source, event.context.Extensions)
				pod := resources.EventLoggerPod(subscriberName)
				client.CreatePodOrFail(pod, lib.WithService(subscriberName))
			}

			// Create triggers.
			for _, event := range test.eventsToReceive {
				triggerName := name("trigger", event.context.Type, event.context.Source, event.context.Extensions)
				subscriberName := name("dumper", event.context.Type, event.context.Source, event.context.Extensions)
				client.CreateTriggerOrFailV1Beta1(triggerName,
					resources.WithSubscriberServiceRefForTriggerV1Beta1(subscriberName),
					resources.WithAttributesTriggerFilterV1Beta1(event.context.Source, event.context.Type, event.context.Extensions),
					resources.WithBrokerV1Beta1(brokerName))

			}

			// Wait for all test resources to become ready before sending the events.
			client.WaitForAllTestResourcesReadyOrFail()

			// Map to save the expected events per dumper so that we can verify the delivery.
			expectedEvents := make(map[string][]string)
			// Map to save the unexpected events per dumper so that we can verify that they weren't delivered.
			unexpectedEvents := make(map[string][]string)
			for _, eventToSend := range test.eventsToSend {
				// Create cloud event.
				// Using event type, source and extensions as part of the body for easier debugging.
				extensionsStr := joinSortedExtensions(eventToSend.Extensions)
				body := fmt.Sprintf(("Body-%s-%s-%s"), eventToSend.Type, eventToSend.Source, extensionsStr)
				cloudEvent := cloudevents.New(
					fmt.Sprintf(`{"msg":%q}`, body),
					cloudevents.WithSource(eventToSend.Source),
					cloudevents.WithType(eventToSend.Type),
					cloudevents.WithExtensions(eventToSend.Extensions),
				)
				// Create sender pod.
				senderPodName := name("sender", eventToSend.Type, eventToSend.Source, eventToSend.Extensions)
				client.SendFakeEventToAddressableOrFail(senderPodName, brokerName, lib.BrokerTypeMeta, cloudEvent)

				// Check on every dumper whether we should expect this event or not, and add its body
				// to the expectedEvents/unexpectedEvents maps.
				for _, eventToReceive := range test.eventsToReceive {
					subscriberName := name("dumper", eventToReceive.context.Type, eventToReceive.context.Source, eventToReceive.context.Extensions)
					if shouldExpectEvent(&eventToSend, &eventToReceive) {
						expectedEvents[subscriberName] = append(expectedEvents[subscriberName], body)
					} else {
						unexpectedEvents[subscriberName] = append(unexpectedEvents[subscriberName], body)
					}
				}
			}

			for _, event := range test.eventsToReceive {
				subscriberName := name("dumper", event.context.Type, event.context.Source, event.context.Extensions)
				if err := client.CheckLog(subscriberName, lib.CheckerContainsAll(expectedEvents[subscriberName])); err != nil {
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

// Checks whether we should expect to receive 'eventToSend' in 'eventReceiver' based on its type and source pattern.
func shouldExpectEvent(eventToSend *eventContext, receiver *eventReceiver) bool {
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
