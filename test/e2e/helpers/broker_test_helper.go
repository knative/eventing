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
	"net/url"
	"sort"
	"strings"
	"testing"

	"knative.dev/eventing/pkg/reconciler/sugar"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
)

type eventTestCase struct {
	Type       string
	Source     string
	Extensions map[string]interface{}
}

// ToString converts the test case to a string to create names for different objects (e.g., triggers, services, etc.).
func (tc eventTestCase) String() string {
	eventType := tc.Type
	eventSource := tc.Source
	extensions := tc.Extensions
	// Pod names need to be lowercase. We might have an eventType as Any, that is why we lowercase them.
	if eventType == v1beta1.TriggerAnyFilter {
		eventType = "testany"
	}
	if eventSource == v1beta1.TriggerAnyFilter {
		eventSource = "testany"
	} else {
		u, _ := url.Parse(eventSource)
		eventSource = strings.Split(u.Host, ".")[0]
	}
	name := strings.ToLower(fmt.Sprintf("%s-%s", eventType, eventSource))
	if len(extensions) > 0 {
		name = strings.ToLower(fmt.Sprintf("%s-%s", name, extensionsToString(extensions)))
	}
	return name
}

// ToEventMatcher converts the test case to the event matcher
func (tc eventTestCase) ToEventMatcher() cetest.EventMatcher {
	var matchers []cetest.EventMatcher
	if tc.Type == v1beta1.TriggerAnyFilter {
		matchers = append(matchers, cetest.ContainsAttributes(spec.Type))
	} else {
		matchers = append(matchers, cetest.HasType(tc.Type))
	}

	if tc.Source == v1beta1.TriggerAnyFilter {
		matchers = append(matchers, cetest.ContainsAttributes(spec.Source))
	} else {
		matchers = append(matchers, cetest.HasSource(tc.Source))
	}

	for k, v := range tc.Extensions {
		if v == v1beta1.TriggerAnyFilter {
			matchers = append(matchers, cetest.ContainsExtensions(k))
		} else {
			matchers = append(matchers, cetest.HasExtension(k, v))
		}
	}

	return cetest.AllOf(matchers...)
}

// BrokerCreator creates a broker and returns its broker name.
// TestBrokerWithManyTriggers will wait for the broker to become ready.
type BrokerCreator func(client *testlib.Client, version string) string

// ChannelBasedBrokerCreator creates a BrokerCreator that creates a broker based on the channel parameter.
func ChannelBasedBrokerCreator(channel metav1.TypeMeta, brokerClass string) BrokerCreator {
	return func(client *testlib.Client, version string) string {
		brokerName := strings.ToLower(channel.Kind)

		// create a ConfigMap used by the broker.
		config := client.CreateBrokerConfigMapOrFail("config-"+brokerName, &channel)

		switch version {
		case "v1":
			client.CreateBrokerV1OrFail(brokerName,
				resources.WithBrokerClassForBrokerV1(brokerClass),
				resources.WithConfigForBrokerV1(config),
			)
		case "v1beta1":
			client.CreateBrokerV1Beta1OrFail(brokerName,
				resources.WithBrokerClassForBrokerV1Beta1(brokerClass),
				resources.WithConfigForBrokerV1Beta1(config),
			)
		default:
			panic("unknown version: " + version)
		}

		return brokerName
	}
}

// If shouldLabelNamespace is set to true this test annotates the testing namespace so that a default broker is created.
// It then binds many triggers with different filtering patterns to the broker created by brokerCreator, and sends
// different events to the broker's address.
// Finally, it verifies that only the appropriate events are routed to the subscribers.
func TestBrokerWithManyTriggers(t *testing.T, brokerCreator BrokerCreator, shouldLabelNamespace bool) {
	const (
		any          = v1beta1.TriggerAnyFilter
		eventType1   = "type1"
		eventType2   = "type2"
		eventSource1 = "http://source1.com"
		eventSource2 = "http://source2.com"
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
	tests := []struct {
		name string
		// These are the event context attributes and extension attributes that will be send.
		eventsToSend []eventTestCase
		// These are the event context attributes and extension attributes that triggers will listen to,
		// to set in the subscriber and services pod
		// The attributes in these test cases will be used as assertions on the receivers
		eventFilters []eventTestCase
		// TriggerFilter with DeprecatedSourceAndType or not
		deprecatedTriggerFilter bool
		// Use v1beta1 trigger
		v1beta1 bool
	}{
		{
			name: "test default broker with many deprecated triggers",
			eventsToSend: []eventTestCase{
				{Type: eventType1, Source: eventSource1},
				{Type: eventType1, Source: eventSource2},
				{Type: eventType2, Source: eventSource1},
				{Type: eventType2, Source: eventSource2},
			},
			eventFilters: []eventTestCase{
				{Type: any, Source: any},
				{Type: eventType1, Source: any},
				{Type: any, Source: eventSource1},
				{Type: eventType1, Source: eventSource1},
			},
			deprecatedTriggerFilter: true,
		}, {
			name: "test default broker with many attribute triggers",
			eventsToSend: []eventTestCase{
				{Type: eventType1, Source: eventSource1},
				{Type: eventType1, Source: eventSource2},
				{Type: eventType2, Source: eventSource1},
				{Type: eventType2, Source: eventSource2},
			},
			eventFilters: []eventTestCase{
				{Type: any, Source: any},
				{Type: eventType1, Source: any},
				{Type: any, Source: eventSource1},
				{Type: eventType1, Source: eventSource1},
			},
			deprecatedTriggerFilter: false,
		}, {
			name: "test default broker with many attribute triggers using v1beta1 trigger",
			eventsToSend: []eventTestCase{
				{Type: eventType1, Source: eventSource1},
				{Type: eventType1, Source: eventSource2},
				{Type: eventType2, Source: eventSource1},
				{Type: eventType2, Source: eventSource2},
			},
			eventFilters: []eventTestCase{
				{Type: any, Source: any},
				{Type: eventType1, Source: any},
				{Type: any, Source: eventSource1},
				{Type: eventType1, Source: eventSource1},
			},
			deprecatedTriggerFilter: false,
			v1beta1:                 true,
		}, {
			name: "test default broker with many attribute and extension triggers",
			eventsToSend: []eventTestCase{
				{Type: eventType1, Source: eventSource1, Extensions: map[string]interface{}{extensionName1: extensionValue1}},
				{Type: eventType1, Source: eventSource1, Extensions: map[string]interface{}{extensionName1: extensionValue1, extensionName2: extensionValue2}},
				{Type: eventType1, Source: eventSource1, Extensions: map[string]interface{}{extensionName2: extensionValue2}},
				{Type: eventType1, Source: eventSource2, Extensions: map[string]interface{}{extensionName1: extensionValue1}},
				{Type: eventType2, Source: eventSource1, Extensions: map[string]interface{}{extensionName1: nonMatchingExtensionValue}},
				{Type: eventType2, Source: eventSource2, Extensions: map[string]interface{}{nonMatchingExtensionName: extensionValue1}},
				{Type: eventType2, Source: eventSource2, Extensions: map[string]interface{}{extensionName1: extensionValue1, extensionName2: extensionValue2}},
				{Type: eventType2, Source: eventSource2, Extensions: map[string]interface{}{extensionName1: extensionValue1, nonMatchingExtensionName: extensionValue2}},
			},
			eventFilters: []eventTestCase{
				{Type: any, Source: any, Extensions: map[string]interface{}{extensionName1: extensionValue1}},
				{Type: any, Source: any, Extensions: map[string]interface{}{extensionName1: extensionValue1, extensionName2: extensionValue2}},
				{Type: any, Source: any, Extensions: map[string]interface{}{extensionName2: extensionValue2}},
				{Type: eventType1, Source: any, Extensions: map[string]interface{}{extensionName1: extensionValue1}},
				{Type: any, Source: any, Extensions: map[string]interface{}{extensionName1: any}},
				{Type: any, Source: eventSource1, Extensions: map[string]interface{}{extensionName1: extensionValue1}},
				{Type: any, Source: eventSource1, Extensions: map[string]interface{}{extensionName1: extensionValue1, extensionName2: extensionValue2}},
			},
			deprecatedTriggerFilter: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := testlib.Setup(t, true)
			defer testlib.TearDown(client)

			if shouldLabelNamespace {
				// Label namespace so that it creates the default broker.
				if err := client.LabelNamespace(map[string]string{sugar.InjectionLabelKey: sugar.InjectionEnabledLabelValue}); err != nil {
					t.Fatalf("Error annotating namespace: %v", err)
				}
			}

			brokerName := brokerCreator(client, "v1beta1")

			// Wait for broker ready.
			client.WaitForResourceReadyOrFail(brokerName, testlib.BrokerTypeMeta)

			if shouldLabelNamespace {
				// Test if namespace reconciler would recreate broker once broker was deleted.
				if err := client.Eventing.EventingV1beta1().Brokers(client.Namespace).Delete(brokerName, &metav1.DeleteOptions{}); err != nil {
					t.Fatalf("Can't delete default broker in namespace: %v", client.Namespace)
				}
				client.WaitForResourceReadyOrFail(brokerName, testlib.BrokerTypeMeta)
			}

			// Let's start event recorders and triggers
			eventTrackers := make(map[string]*recordevents.EventInfoStore, len(test.eventFilters))
			for _, event := range test.eventFilters {
				// Create event recorder pod and service
				subscriberName := "dumper-" + event.String()
				eventTracker, _ := recordevents.StartEventRecordOrFail(client, subscriberName)
				eventTrackers[subscriberName] = eventTracker
				// Create trigger.
				triggerName := "trigger-" + event.String()
				client.CreateTriggerOrFailV1Beta1(triggerName,
					resources.WithSubscriberServiceRefForTriggerV1Beta1(subscriberName),
					resources.WithAttributesTriggerFilterV1Beta1(event.Source, event.Type, event.Extensions),
					resources.WithBrokerV1Beta1(brokerName),
				)
			}
			// Wait for all test resources to become ready before sending the events.
			client.WaitForAllTestResourcesReadyOrFail()

			// Map to save the expected matchers per dumper so that we can verify the delivery.
			expectedMatchers := make(map[string][]recordevents.EventInfoMatcher)
			// Map to save the unexpected matchers per dumper so that we can verify that they weren't delivered.
			unexpectedMatchers := make(map[string][]recordevents.EventInfoMatcher)

			// Now we need to send events and populate the expectedMatcher/unexpectedMatchers map,
			// in order to assert if I correctly receive only the expected events
			for _, eventTestCase := range test.eventsToSend {
				// Create cloud event.
				// Using event type, source and extensions as part of the body for easier debugging.
				eventToSend := cloudevents.NewEvent()
				eventToSend.SetID(uuid.New().String())
				eventToSend.SetType(eventTestCase.Type)
				eventToSend.SetSource(eventTestCase.Source)
				for k, v := range eventTestCase.Extensions {
					eventToSend.SetExtension(k, v)
				}

				data := fmt.Sprintf(`{"msg":"%s"}`, eventTestCase.String())
				if err := eventToSend.SetData(cloudevents.ApplicationJSON, []byte(data)); err != nil {
					t.Fatalf("Cannot set the payload of the event: %s", err.Error())
				}

				// Send event
				senderPodName := "sender-" + eventTestCase.String()
				client.SendEventToAddressable(senderPodName, brokerName, testlib.BrokerTypeMeta, eventToSend)

				// Sent event matcher
				sentEventMatcher := cetest.AllOf(
					cetest.HasId(eventToSend.ID()),
					eventTestCase.ToEventMatcher(),
				)

				// Check on every dumper whether we should expect this event or not
				for _, eventFilter := range test.eventFilters {
					subscriberName := "dumper-" + eventFilter.String()

					if eventFilter.ToEventMatcher()(eventToSend) == nil {
						// This filter should match this event
						expectedMatchers[subscriberName] = append(
							expectedMatchers[subscriberName],
							recordevents.MatchEvent(sentEventMatcher),
						)
					} else {
						// This filter should not match this event
						unexpectedMatchers[subscriberName] = append(
							unexpectedMatchers[subscriberName],
							recordevents.MatchEvent(sentEventMatcher),
						)
					}
				}
			}

			// Let's check that all expected matchers are fulfilled
			for subscriberName, matchers := range expectedMatchers {
				eventTracker := eventTrackers[subscriberName]

				for _, matcher := range matchers {
					// One match per event is enough
					eventTracker.AssertAtLeast(1, matcher)
				}
			}

			// Let's check the unexpected matchers
			// NOTE: this check is not really robust because we could receive
			// an unexpected event after the check is done
			for subscriberName, matchers := range unexpectedMatchers {
				eventTracker := eventTrackers[subscriberName]

				for _, matcher := range matchers {
					eventTracker.AssertNot(matcher)
				}
			}
		})
	}
}

func extensionsToString(extensions map[string]interface{}) string {
	// Sort extension keys
	sortedExtensionNames := make([]string, 0)
	for k := range extensions {
		sortedExtensionNames = append(sortedExtensionNames, k)
	}
	sort.Strings(sortedExtensionNames)

	// Write map as string
	var sb strings.Builder
	for _, sortedExtensionName := range sortedExtensionNames {
		sb.WriteString("-")
		sb.WriteString(sortedExtensionName)
		sb.WriteString("-")
		vStr := fmt.Sprintf("%v", extensions[sortedExtensionName])
		if vStr == v1beta1.TriggerAnyFilter {
			vStr = "testany"
		}
		sb.WriteString(vStr)
	}
	return sb.String()
}
