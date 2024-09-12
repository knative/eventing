/*
Copyright 2022 The Knative Authors

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
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/state"

	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/test/rekt/features"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/channel"
	"knative.dev/eventing/test/rekt/resources/subscription"
	"knative.dev/eventing/test/rekt/resources/trigger"

	v1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/network"
	"knative.dev/pkg/ptr"

	"knative.dev/reconciler-test/pkg/eventshub"
	eventasssert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"
)

func ManyTriggers() *feature.FeatureSet {
	fs := &feature.FeatureSet{Name: "Broker with many triggers"}

	// Construct different type, source and extensions of events
	any := eventingv1.TriggerAnyFilter
	eventType1 := "type1"
	eventType2 := "type2"
	eventSource1 := "http://source1.com"
	eventSource2 := "http://source2.com"
	// Be careful with the length of extension name and values,
	// we use extension name and value as a part of the name of resources like subscriber and trigger,
	// the maximum characters allowed of resource name is 63
	extensionName1 := "extname1"
	extensionValue1 := "extval1"
	extensionName2 := "extname2"
	extensionValue2 := "extvalue2"
	nonMatchingExtensionName := "nonmatchingextname"
	nonMatchingExtensionValue := "nonmatchingextval"

	eventFilters1 := make(map[string]eventTestCase)
	eventFilters1["dumper-1"] = newEventTestCase(any, any)
	eventFilters1["dumper-12"] = newEventTestCase(eventType1, any)
	eventFilters1["dumper-123"] = newEventTestCase(any, eventSource1)
	eventFilters1["dumper-1234"] = newEventTestCase(eventType1, eventSource1)

	eventFilters2 := make(map[string]eventTestCase)
	eventFilters2["dumper-12345"] = newEventTestCaseWithExtensions(any, any, map[string]interface{}{extensionName1: extensionValue1})
	eventFilters2["dumper-123456"] = newEventTestCaseWithExtensions(any, any, map[string]interface{}{extensionName1: extensionValue1, extensionName2: extensionValue2})
	eventFilters2["dumper-1234567"] = newEventTestCaseWithExtensions(any, any, map[string]interface{}{extensionName2: extensionValue2})
	eventFilters2["dumper-654321"] = newEventTestCaseWithExtensions(eventType1, any, map[string]interface{}{extensionName1: extensionValue1})
	eventFilters2["dumper-54321"] = newEventTestCaseWithExtensions(any, any, map[string]interface{}{extensionName1: any})
	eventFilters2["dumper-4321"] = newEventTestCaseWithExtensions(any, eventSource1, map[string]interface{}{extensionName1: extensionValue1})
	eventFilters2["dumper-321"] = newEventTestCaseWithExtensions(any, eventSource1, map[string]interface{}{extensionName1: extensionValue1, extensionName2: extensionValue2})
	eventFilters2["dumper-21"] = newEventTestCaseWithExtensions(any, eventSource2, map[string]interface{}{extensionName1: extensionValue1, extensionName2: extensionValue1})

	tests := []struct {
		name string
		// These are the event context attributes and extension attributes that will be send.
		eventsToSend []eventTestCase
		// These are the event context attributes and extension attributes that triggers will listen to
		// This map is to configure sink and corresponding filter to construct trigger
		eventFilters map[string]eventTestCase
	}{
		{
			name: "test default broker with many attribute triggers",
			eventsToSend: []eventTestCase{
				{Type: eventType1, Source: eventSource1},
				{Type: eventType1, Source: eventSource2},
				{Type: eventType2, Source: eventSource1},
				{Type: eventType2, Source: eventSource2},
			},
			eventFilters: eventFilters1,
		},
		{
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
			eventFilters: eventFilters2,
		},
	}

	for _, testcase := range tests {

		testcase := testcase // capture variable
		f := feature.NewFeatureNamed(testcase.name)

		// Create the broker
		brokerName := feature.MakeRandomK8sName("broker")

		f.Setup("install broker, sinks and triggers", func(ctx context.Context, t feature.T) {
			broker.Install(brokerName, broker.WithEnvConfig()...)(ctx, t)

			for sink, eventFilter := range testcase.eventFilters {
				eventshub.Install(sink, eventshub.StartReceiver)(ctx, t)

				filter := eventingv1.TriggerFilterAttributes{
					"type":   eventFilter.Type,
					"source": eventFilter.Source,
				}

				// Point the Trigger subscriber to the sink svc.
				cfg := []manifest.CfgFn{
					trigger.WithSubscriber(service.AsKReference(sink), ""),
					trigger.WithFilter(filter),
					trigger.WithExtensions(eventFilter.Extensions),
					trigger.WithBrokerName(brokerName),
				}

				trigger.Install(sink, cfg...)(ctx, t)
			}

			broker.IsReady(brokerName)(ctx, t)
			broker.IsAddressable(brokerName)(ctx, t)

			for sink := range testcase.eventFilters {
				trigger.IsReady(sink)(ctx, t)
			}
		})

		for _, event := range testcase.eventsToSend {
			eventToSend := cloudevents.NewEvent()
			eventToSend.SetID(event.toID())
			eventToSend.SetType(event.Type)
			eventToSend.SetSource(event.Source)
			for k, v := range event.Extensions {
				eventToSend.SetExtension(k, v)
			}
			data := fmt.Sprintf(`{"msg":"%s"}`, eventToSend.ID())
			eventToSend.SetData(cloudevents.ApplicationJSON, []byte(data))

			source := feature.MakeRandomK8sName("source")
			f.Requirement("install source", eventshub.Install(
				source,
				eventshub.StartSenderToResource(broker.GVR(), brokerName),
				eventshub.InputEvent(eventToSend),
			))

			f.Assert("source sent event", eventasssert.OnStore(source).
				MatchSentEvent(test.HasId(eventToSend.ID())).
				AtLeast(1),
			)

			for sink, eventFilter := range testcase.eventFilters {
				sink := sink                      // capture variable
				matcher := event.toEventMatcher() // capture variable

				// Check on every dumper whether we should expect this event or not
				if eventFilter.toEventMatcher()(eventToSend) == nil {
					f.Assert(fmt.Sprintf("%s receive event %s", sink, eventToSend.ID()), func(ctx context.Context, t feature.T) {
						eventasssert.OnStore(sink).
							Match(features.HasKnNamespaceHeader(environment.FromContext(ctx).Namespace())).
							MatchReceivedEvent(test.HasId(eventToSend.ID())).
							MatchReceivedEvent(matcher).
							AtLeast(1)(ctx, t)
					})
				}
			}
		}

		fs.Features = append(fs.Features, f)
	}

	return fs
}

func BrokerWorkFlowWithTransformation() *feature.FeatureSet {
	createSubscriberFn := func(ref *v1.KReference, uri string) manifest.CfgFn {
		return subscription.WithSubscriber(ref, uri, "")
	}
	fs := &feature.FeatureSet{
		Name: "Knative Broker - Transformation - Channel flow and Trigger event flow",

		Features: []*feature.Feature{
			brokerChannelFlowWithTransformation(createSubscriberFn),
			brokerEventTransformationForTrigger(),
		},
	}
	return fs
}

/*
BrokerChannelFlowWithTransformation tests the following topology:
	------------- ----------------------
	|           | |                    |
	v	       | v                    |
EventSource ---> Broker ---> Trigger1 -------> Sink1(Transformation)
	|
	|
	|-------> Trigger2 -------> Sink2(Logger1)
	|
	|
	|-------> Trigger3 -------> Channel --------> Subscription --------> Sink3(Logger2)
Explanation:
Trigger1 filters the orignal event and transforms it to a new event,
Trigger2 logs all events,
Trigger3 filters the transformed event and sends it to Channel.
*/

func brokerChannelFlowWithTransformation(createSubscriberFn func(ref *v1.KReference, uri string) manifest.CfgFn) *feature.Feature {
	f := feature.NewFeatureNamed("Broker topology of transformation")

	source := feature.MakeRandomK8sName("source")

	sink1 := feature.MakeRandomK8sName("sink1")
	sink2 := feature.MakeRandomK8sName("sink2")
	sink3 := feature.MakeRandomK8sName("sink3")

	trigger1 := feature.MakeRandomK8sName("trigger1")
	trigger2 := feature.MakeRandomK8sName("trigger2")
	trigger3 := feature.MakeRandomK8sName("trigger3")

	// Construct original cloudevent message
	eventType := "type1"
	eventSource := "http://source1.com"
	eventBody := `{"msg":"e2e-brokerchannel-body"}`
	// Construct cloudevent message after transformation
	transformedEventType := "type2"
	transformedEventSource := "http://source2.com"
	transformedBody := `{"msg":"transformed body"}`
	// Construct eventToSend
	eventToSend := cloudevents.NewEvent()
	eventToSend.SetID(uuid.New().String())
	eventToSend.SetType(eventType)
	eventToSend.SetSource(eventSource)
	eventToSend.SetData(cloudevents.ApplicationJSON, []byte(eventBody))

	//Install the broker
	brokerName := feature.MakeRandomK8sName("broker")
	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("broker is ready", broker.IsReady(brokerName))
	f.Setup("broker is addressable", broker.IsAddressable(brokerName))

	f.Setup("install sink1", eventshub.Install(sink1,
		eventshub.ReplyWithTransformedEvent(transformedEventType, transformedEventSource, transformedBody),
		eventshub.StartReceiver),
	)
	f.Setup("install sink2", eventshub.Install(sink2, eventshub.StartReceiver))
	f.Setup("install sink3", eventshub.Install(sink3, eventshub.StartReceiver))

	// filter1 filters the original events
	filter1 := eventingv1.TriggerFilterAttributes{
		"type":   eventType,
		"source": eventSource,
	}
	// filter2 filters all events
	filter2 := eventingv1.TriggerFilterAttributes{
		"type": eventingv1.TriggerAnyFilter,
	}
	// filter3 filters events after transformation
	filter3 := eventingv1.TriggerFilterAttributes{
		"type":   transformedEventType,
		"source": transformedEventSource,
	}

	// Install the trigger1 point to Broker and transform the original events to new events
	f.Setup("install trigger1", trigger.Install(
		trigger1,
		trigger.WithBrokerName(brokerName),
		trigger.WithFilter(filter1),
		trigger.WithSubscriber(service.AsKReference(sink1), ""),
	))
	f.Setup("trigger1 goes ready", trigger.IsReady(trigger1))
	// Install the trigger2 point to Broker to filter all the events
	f.Setup("install trigger2", trigger.Install(
		trigger2,
		trigger.WithBrokerName(brokerName),
		trigger.WithFilter(filter2),
		trigger.WithSubscriber(service.AsKReference(sink2), ""),
	))
	f.Setup("trigger2 goes ready", trigger.IsReady(trigger2))

	// Install the channel and corresponding subscription point to sink3
	channelName := feature.MakeRandomK8sName("channel")
	f.Setup("install channel", channel.Install(channelName,
		channel.WithTemplate(),
	))
	sub := feature.MakeRandomK8sName("subscription")
	f.Setup("install subscription", subscription.Install(sub,
		subscription.WithChannel(channel.AsRef(channelName)),
		createSubscriberFn(service.AsKReference(sink3), ""),
	))
	f.Setup("subscription is ready", subscription.IsReady(sub))
	f.Setup("channel is ready", channel.IsReady(channelName))

	// Install the trigger3 point to Broker to filter the events after transformation point to channel
	f.Setup("install trigger3", trigger.Install(
		trigger3,
		trigger.WithBrokerName(brokerName),
		trigger.WithFilter(filter3),
		trigger.WithSubscriber(channel.AsRef(channelName), ""),
	))
	f.Setup("trigger3 goes ready", trigger.IsReady(trigger3))

	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(eventToSend),
	))

	eventMatcher := eventasssert.MatchEvent(
		test.HasSource(eventSource),
		test.HasType(eventType),
		test.HasData([]byte(eventBody)),
	)
	transformEventMatcher := eventasssert.MatchEvent(
		test.HasSource(transformedEventSource),
		test.HasType(transformedEventType),
		test.HasData([]byte(transformedBody)),
	)

	f.Stable("(Trigger1 point to) sink1 has all the events").
		Must("delivers original events",
			eventasssert.OnStore(sink1).Match(eventMatcher).AtLeast(1))

	f.Stable("(Trigger2 point to) sink2 has all the events").
		Must("delivers original events",
			eventasssert.OnStore(sink2).Match(eventMatcher).AtLeast(1)).
		Must("delivers transformation events",
			eventasssert.OnStore(sink2).Match(transformEventMatcher).AtLeast(1))

	f.Stable("(Trigger3 point to) Channel's subscriber just has events after transformation").
		Must("delivers transformation events",
			eventasssert.OnStore(sink3).Match(transformEventMatcher).AtLeast(1)).
		Must("delivers original events",
			eventasssert.OnStore(sink3).Match(eventMatcher).Not())

	return f
}

/*
BrokerEventTransformationForTrigger tests the following scenario:

	            5                 4
	      ------------- ----------------------
	      |           | |                    |
	1     v	 2     | v        3           |

EventSource ---> Broker ---> Trigger1 -------> Sink1(Transformation)

	|
	| 6                   7
	|-------> Trigger2 -------> Sink2(Logger)

Note: the number denotes the sequence of the event that flows in this test case.
*/
func brokerEventTransformationForTrigger() *feature.Feature {
	f := feature.NewFeatureNamed("Broker event transformation for trigger")
	config := BrokerEventTransformationForTriggerSetup(f)
	BrokerEventTransformationForTriggerAssert(f, config)
	return f
}

type brokerEventTransformationConfig struct {
	Broker           string
	Sink1            string
	Sink2            string
	EventToSend      cloudevents.Event
	TransformedEvent cloudevents.Event
}

func BrokerEventTransformationForTriggerSetup(f *feature.Feature) brokerEventTransformationConfig {
	sink1 := feature.MakeRandomK8sName("sink1")
	sink2 := feature.MakeRandomK8sName("sink2")

	trigger1 := feature.MakeRandomK8sName("trigger1")
	trigger2 := feature.MakeRandomK8sName("trigger2")

	// Construct original cloudevent message
	eventToSend := cloudevents.NewEvent()
	eventToSend.SetType("type1")
	eventToSend.SetSource("http://source1.com")
	eventToSend.SetData(cloudevents.ApplicationJSON, []byte(`{"msg":"e2e-brokerchannel-body"}`))

	// Construct cloudevent message after transformation
	transformedEvent := cloudevents.NewEvent()
	transformedEvent.SetType("type2")
	transformedEvent.SetSource("http://source2.com")
	transformedEvent.SetData(cloudevents.ApplicationJSON, []byte(`{"msg":"transformed body"}`))

	//Install the broker
	brokerName := feature.MakeRandomK8sName("broker")
	f.Setup("Set context variables", func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, "brokerName", brokerName)
		state.SetOrFail(ctx, t, "sink1", sink1)
		state.SetOrFail(ctx, t, "sink2", sink2)
	})
	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("broker is ready", broker.IsReady(brokerName))
	f.Setup("broker is addressable", broker.IsAddressable(brokerName))

	f.Setup("install sink1", eventshub.Install(sink1,
		eventshub.ReplyWithTransformedEvent(transformedEvent.Type(), transformedEvent.Source(), string(transformedEvent.Data())),
		eventshub.StartReceiver),
	)
	f.Setup("install sink2", eventshub.Install(sink2, eventshub.StartReceiver))

	// filter1 filters the original events
	filter1 := eventingv1.TriggerFilterAttributes{
		"type":   eventToSend.Type(),
		"source": eventToSend.Source(),
	}
	// filter2 filters events after transformation
	filter2 := eventingv1.TriggerFilterAttributes{
		"type":   transformedEvent.Type(),
		"source": transformedEvent.Source(),
	}

	// Install the trigger1 point to Broker and transform the original events to new events
	f.Setup("install trigger1", trigger.Install(
		trigger1,
		trigger.WithBrokerName(brokerName),
		trigger.WithFilter(filter1),
		trigger.WithSubscriber(service.AsKReference(sink1), ""),
	))
	f.Setup("trigger1 goes ready", trigger.IsReady(trigger1))
	// Install the trigger2 point to Broker to filter all the events
	f.Setup("install trigger2", trigger.Install(
		trigger2,
		trigger.WithBrokerName(brokerName),
		trigger.WithFilter(filter2),
		trigger.WithSubscriber(service.AsKReference(sink2), ""),
	))
	f.Setup("trigger2 goes ready", trigger.IsReady(trigger2))

	return brokerEventTransformationConfig{
		Broker:           brokerName,
		Sink1:            sink1,
		Sink2:            sink2,
		EventToSend:      eventToSend,
		TransformedEvent: transformedEvent,
	}
}

func BrokerEventTransformationForTriggerAssert(f *feature.Feature,
	cfg brokerEventTransformationConfig) {

	source := feature.MakeRandomK8sName("source")

	// Set new ID every time we send event to allow calling this function repeatedly
	cfg.EventToSend.SetID(uuid.New().String())
	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResource(broker.GVR(), cfg.Broker),
		eventshub.InputEvent(cfg.EventToSend),
	))

	eventMatcher := eventasssert.MatchEvent(
		test.HasId(cfg.EventToSend.ID()),
		test.HasSource(cfg.EventToSend.Source()),
		test.HasType(cfg.EventToSend.Type()),
		test.HasData(cfg.EventToSend.Data()),
	)
	transformEventMatcher := eventasssert.MatchEvent(
		test.HasSource(cfg.TransformedEvent.Source()),
		test.HasType(cfg.TransformedEvent.Type()),
		test.HasData(cfg.TransformedEvent.Data()),
	)

	f.Stable("Trigger has filtered all transformed events").
		Must("trigger 1 delivers original events",
			eventasssert.OnStore(cfg.Sink1).Match(eventMatcher).AtLeast(1)).
		Must("trigger 1 does not deliver transformed events",
			eventasssert.OnStore(cfg.Sink1).Match(transformEventMatcher).Not()).
		Must("trigger 2 delivers transformed events",
			eventasssert.OnStore(cfg.Sink2).Match(transformEventMatcher).AtLeast(1)).
		Must("trigger 2 does not deliver original events",
			eventasssert.OnStore(cfg.Sink2).Match(eventMatcher).Not())
}

func BrokerPreferHeaderCheck() *feature.Feature {
	f := feature.NewFeatureNamed("Broker PreferHeader Check")

	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	via := feature.MakeRandomK8sName("via")

	eventSource := "source1"
	eventType := "type1"
	eventBody := `{"msg":"test msg"}`
	event := cloudevents.NewEvent()
	event.SetID(uuid.New().String())
	event.SetType(eventType)
	event.SetSource(eventSource)
	event.SetData(cloudevents.ApplicationJSON, []byte(eventBody))

	//Install the broker
	brokerName := feature.MakeRandomK8sName("broker")
	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Requirement("broker is ready", broker.IsReady(brokerName))
	f.Requirement("broker is addressable", broker.IsAddressable(brokerName))

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	// Point the Trigger subscriber to the sink svc.
	cfg := []manifest.CfgFn{trigger.WithSubscriber(service.AsKReference(sink), ""), trigger.WithBrokerName(brokerName)}

	// Install the trigger
	f.Setup("install trigger", trigger.Install(via, cfg...))
	f.Setup("trigger goes ready", trigger.IsReady(via))

	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
	))

	f.Stable("test message without explicit prefer header should have the header").
		Must("delivers events",
			eventasssert.OnStore(sink).Match(
				eventasssert.HasAdditionalHeader("Prefer", "reply"),
			).AtLeast(1))

	return f
}

func BrokerRedelivery() *feature.FeatureSet {
	fs := &feature.FeatureSet{
		Name: "Knative Broker - Redelivery - with different sequences",
		Features: []*feature.Feature{
			brokerRedeliveryFibonacci(5),
			brokerRedeliveryDropN(5, 5),
		},
	}

	return fs
}

func brokerRedeliveryFibonacci(retryNum int32) *feature.Feature {
	f := feature.NewFeatureNamed("Broker reply with a bad status code following the fibonacci sequence")

	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	via := feature.MakeRandomK8sName("via")

	eventSource := "source1"
	eventType := "type1"
	eventBody := `{"msg":"fibonacci"}`
	event := cloudevents.NewEvent()
	event.SetID(uuid.New().String())
	event.SetType(eventType)
	event.SetSource(eventSource)
	event.SetData(cloudevents.ApplicationJSON, []byte(eventBody))

	//Install the broker
	brokerName := feature.MakeRandomK8sName("broker")

	exp := duckv1.BackoffPolicyLinear
	brokerConfig := append(broker.WithEnvConfig(), broker.WithRetry(retryNum, &exp, ptr.String("PT1S")))
	f.Setup("install broker", broker.Install(brokerName, brokerConfig...))
	f.Requirement("broker is ready", broker.IsReady(brokerName))
	f.Requirement("broker is addressable", broker.IsAddressable(brokerName))

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	// Point the Trigger subscriber to the sink svc.
	cfg := []manifest.CfgFn{trigger.WithSubscriber(service.AsKReference(sink), ""), trigger.WithBrokerName(brokerName)}

	// Install the trigger
	f.Setup("install trigger", trigger.Install(via, cfg...))
	f.Setup("trigger goes ready", trigger.IsReady(via))

	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
		eventshub.FibonacciDrop,
	))

	f.Stable("Broker Redelivery following the fibonacci sequence").
		Must("delivers events",
			eventasssert.OnStore(sink).Match(
				eventasssert.MatchKind(eventasssert.EventReceived),
				eventasssert.MatchEvent(
					test.HasSource(eventSource),
					test.HasType(eventType),
					test.HasData([]byte(eventBody)),
				),
			).AtLeast(1))

	return f
}

func brokerRedeliveryDropN(retryNum int32, dropNum uint) *feature.Feature {
	f := feature.NewFeatureNamed("Broker reply with a bad status code to the first n events")

	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	via := feature.MakeRandomK8sName("via")

	eventSource := "source2"
	eventType := "type2"
	eventBody := `{"msg":"DropN"}`
	event := cloudevents.NewEvent()
	event.SetID(uuid.New().String())
	event.SetType(eventType)
	event.SetSource(eventSource)
	event.SetData(cloudevents.ApplicationJSON, []byte(eventBody))

	//Install the broker
	brokerName := feature.MakeRandomK8sName("broker")
	exp := duckv1.BackoffPolicyLinear
	brokerConfig := append(broker.WithEnvConfig(), broker.WithRetry(retryNum, &exp, ptr.String("PT1S")))

	f.Setup("install broker", broker.Install(brokerName, brokerConfig...))
	f.Requirement("broker is ready", broker.IsReady(brokerName))
	f.Requirement("broker is addressable", broker.IsAddressable(brokerName))

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	// Point the Trigger subscriber to the sink svc.
	cfg := []manifest.CfgFn{trigger.WithSubscriber(service.AsKReference(sink), ""), trigger.WithBrokerName(brokerName)}

	// Install the trigger
	f.Setup("install trigger", trigger.Install(via, cfg...))
	f.Setup("trigger goes ready", trigger.IsReady(via))

	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
		eventshub.DropFirstN(dropNum),
	))

	f.Stable("Broker Redelivery failed the first n events").
		Must("delivers events",
			func(ctx context.Context, t feature.T) {
				eventasssert.OnStore(sink).
					Match(features.HasKnNamespaceHeader(environment.FromContext(ctx).Namespace())).
					Match(
						eventasssert.MatchKind(eventasssert.EventReceived),
						eventasssert.MatchEvent(
							test.HasSource(eventSource),
							test.HasType(eventType),
							test.HasData([]byte(eventBody)),
						),
					).
					AtLeast(1)(ctx, t)
			})

	return f
}

func BrokerDeadLetterSinkExtensions() *feature.FeatureSet {
	fs := &feature.FeatureSet{
		Name: "Knative Broker - DeadLetterSink - with Extensions",

		Features: []*feature.Feature{
			brokerSubscriberUnreachable(),
			brokerSubscriberErrorNodata(),
			brokerSubscriberErrorWithdata(),
		},
	}
	return fs
}

func brokerSubscriberUnreachable() *feature.Feature {
	f := feature.NewFeatureNamed("Broker Subscriber Unreachable")

	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	triggerName := feature.MakeRandomK8sName("triggerName")

	eventSource := "source1"
	eventType := "type1"
	eventBody := `{"msg":"test msg"}`
	event := cloudevents.NewEvent()
	event.SetID(uuid.New().String())
	event.SetType(eventType)
	event.SetSource(eventSource)
	event.SetData(cloudevents.ApplicationJSON, []byte(eventBody))

	subscriberUri := fmt.Sprintf("http://fake.svc.%s", network.GetClusterDomainName())

	//Install the broker
	brokerName := feature.MakeRandomK8sName("broker")
	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Requirement("broker is ready", broker.IsReady(brokerName))
	f.Requirement("broker is addressable", broker.IsAddressable(brokerName))

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	// Install the trigger and Point the Trigger subscriber to the sink svc.
	f.Setup("install trigger", trigger.Install(
		triggerName,
		trigger.WithBrokerName(brokerName),
		trigger.WithSubscriber(nil, subscriberUri),
		trigger.WithDeadLetterSink(service.AsKReference(sink), ""),
	))
	f.Setup("trigger goes ready", trigger.IsReady(triggerName))

	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
	))

	f.Assert("Receives dls extensions when subscriber is unreachable",
		func(ctx context.Context, t feature.T) {
			eventasssert.OnStore(sink).
				Match(features.HasKnNamespaceHeader(environment.FromContext(ctx).Namespace())).
				MatchEvent(
					test.HasExtension("knativeerrordest", subscriberUri),
				).
				AtLeast(1)(ctx, t)
		},
	)
	return f
}

func brokerSubscriberErrorNodata() *feature.Feature {
	f := feature.NewFeatureNamed("Broker Subscriber Error Nodata")

	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	failer := feature.MakeRandomK8sName("failer")
	triggerName := feature.MakeRandomK8sName("triggerName")

	eventSource := "source1"
	eventType := "type1"
	eventBody := `{"msg":"test msg"}`
	event := cloudevents.NewEvent()
	event.SetID(uuid.New().String())
	event.SetType(eventType)
	event.SetSource(eventSource)
	event.SetData(cloudevents.ApplicationJSON, []byte(eventBody))

	//Install the broker
	brokerName := feature.MakeRandomK8sName("broker")
	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Requirement("broker is ready", broker.IsReady(brokerName))
	f.Requirement("broker is addressable", broker.IsAddressable(brokerName))

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	f.Setup("install failing receiver", eventshub.Install(failer,
		eventshub.StartReceiver,
		eventshub.DropFirstN(1),
		eventshub.DropEventsResponseCode(422),
	))

	// Install the trigger and Point the Trigger subscriber to the sink svc.
	f.Setup("install trigger", trigger.Install(
		triggerName,
		trigger.WithBrokerName(brokerName),
		trigger.WithSubscriber(service.AsKReference(failer), ""),
		trigger.WithDeadLetterSink(service.AsKReference(sink), ""),
	))
	f.Setup("trigger goes ready", trigger.IsReady(triggerName))

	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
	))

	f.Assert("Receives dls extensions without errordata", assertEnhancedWithKnativeErrorExtensions(
		sink,
		func(ctx context.Context) test.EventMatcher {
			failerAddress, _ := service.Address(ctx, failer)
			return test.HasExtension("knativeerrordest", failerAddress.URL.String())
		},
		func(ctx context.Context) test.EventMatcher {
			return test.HasExtension("knativeerrorcode", "422")
		},
	))

	return f
}

func brokerSubscriberErrorWithdata() *feature.Feature {
	f := feature.NewFeatureNamed("Broker Subscriber Error With data encoded")

	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	failer := feature.MakeRandomK8sName("failer")
	triggerName := feature.MakeRandomK8sName("triggerName")

	eventSource := "source1"
	eventType := "type1"
	eventBody := `{"msg":"test msg"}`
	event := cloudevents.NewEvent()
	event.SetID(uuid.New().String())
	event.SetType(eventType)
	event.SetSource(eventSource)
	event.SetData(cloudevents.ApplicationJSON, []byte(eventBody))

	//Install the broker
	brokerName := feature.MakeRandomK8sName("broker")
	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Requirement("broker is ready", broker.IsReady(brokerName))
	f.Requirement("broker is addressable", broker.IsAddressable(brokerName))

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	errorData := `{ "message": "catastrophic failure" }`
	f.Setup("install failing receiver", eventshub.Install(failer,
		eventshub.StartReceiver,
		eventshub.DropFirstN(1),
		eventshub.DropEventsResponseCode(422),
		eventshub.DropEventsResponseBody(errorData),
	))

	// Install the trigger and Point the Trigger subscriber to the sink svc.
	f.Setup("install trigger", trigger.Install(
		triggerName,
		trigger.WithBrokerName(brokerName),
		trigger.WithSubscriber(service.AsKReference(failer), ""),
		trigger.WithDeadLetterSink(service.AsKReference(sink), ""),
	))
	f.Setup("trigger goes ready", trigger.IsReady(triggerName))

	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
	))

	f.Assert("Receives dls extensions with errordata", assertEnhancedWithKnativeErrorExtensions(
		sink,
		func(ctx context.Context) test.EventMatcher {
			failerAddress, _ := service.Address(ctx, failer)
			return test.HasExtension("knativeerrordest", failerAddress.URL.String())
		},
		func(ctx context.Context) test.EventMatcher {
			return test.HasExtension("knativeerrorcode", "422")
		},
		func(ctx context.Context) test.EventMatcher {
			return test.HasExtension("knativeerrordata", base64.StdEncoding.EncodeToString([]byte(errorData)))
		},
	))

	return f
}

func assertEnhancedWithKnativeErrorExtensions(sinkName string, matcherfns ...func(ctx context.Context) test.EventMatcher) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		matchers := make([]test.EventMatcher, len(matcherfns))
		for i, fn := range matcherfns {
			matchers[i] = fn(ctx)
		}
		_ = eventshub.StoreFromContext(ctx, sinkName).AssertExact(
			ctx,
			t,
			1,
			eventasssert.MatchKind(eventshub.EventReceived),
			eventasssert.MatchEvent(matchers...),
		)
	}
}

func BrokerDeliverLongMessage() *feature.FeatureSet {
	fs := &feature.FeatureSet{
		Name: "Knative Broker - DeadLetterSink - with Extensions",

		Features: []*feature.Feature{
			brokerSubscriberLongMessage(),
		},
	}
	return fs
}

func brokerSubscriberLongMessage() *feature.Feature {
	f := feature.NewFeatureNamed("Broker Subscriber with long data message")

	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	triggerName := feature.MakeRandomK8sName("triggerName")

	eventSource := "source1"
	eventType := "type1"
	eventBody := strings.Repeat("X", 36864)
	event := cloudevents.NewEvent()
	event.SetID(uuid.New().String())
	event.SetType(eventType)
	event.SetSource(eventSource)
	event.SetData(cloudevents.TextPlain, []byte(eventBody))

	//Install the broker
	brokerName := feature.MakeRandomK8sName("broker")
	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Requirement("broker is ready", broker.IsReady(brokerName))
	f.Requirement("broker is addressable", broker.IsAddressable(brokerName))

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	// Install the trigger and Point the Trigger subscriber to the sink svc.
	f.Setup("install trigger", trigger.Install(
		triggerName,
		trigger.WithBrokerName(brokerName),
		trigger.WithSubscriber(service.AsKReference(sink), ""),
	))
	f.Setup("trigger goes ready", trigger.IsReady(triggerName))

	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
	))

	f.Assert("receive long event on sink exactly once",
		eventasssert.OnStore(sink).
			MatchEvent(test.HasData([]byte(eventBody))).
			Exact(1),
	)
	return f
}

/*
Following test sends an event to the first sink, Sink1, which will send a long response destined to Sink2.
The test will assert that the long response is received by Sink2
EventSource ---> Broker ---> Trigger1 ---> Sink1(Transformation) ---> Trigger2 --> Sink2
*/

func BrokerDeliverLongResponseMessage() *feature.FeatureSet {
	fs := &feature.FeatureSet{
		Name: "Knative Broker - Long Response Message",

		Features: []*feature.Feature{
			brokerSubscriberLongResponseMessage(),
		},
	}
	return fs
}

func brokerSubscriberLongResponseMessage() *feature.Feature {
	f := feature.NewFeatureNamed("Broker, chain of Triggers, long response message from first subscriber")

	source := feature.MakeRandomK8sName("source")
	sink1 := feature.MakeRandomK8sName("sink1")
	sink2 := feature.MakeRandomK8sName("sink2")
	trigger1 := feature.MakeRandomK8sName("trigger1")
	trigger2 := feature.MakeRandomK8sName("trigger2")

	eventSource1 := "source1"
	eventSource2 := "source2"
	eventType1 := "type1"
	eventType2 := "type2"
	eventBody := `{"msg":"eventBody"}`
	transformedEventBody := `{"msg":"` + strings.Repeat("X", 36000) + `"}`
	event := cloudevents.NewEvent()
	event.SetID(uuid.New().String())
	event.SetType(eventType1)
	event.SetSource(eventSource1)
	event.SetData(cloudevents.TextPlain, []byte(eventBody))

	//Install the broker
	brokerName := feature.MakeRandomK8sName("broker")
	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Requirement("broker is ready", broker.IsReady(brokerName))
	f.Requirement("broker is addressable", broker.IsAddressable(brokerName))

	// Sink1 will transform the event so it can be filtered by Trigger2
	f.Setup("install sink1", eventshub.Install(
		sink1,
		eventshub.ReplyWithTransformedEvent(eventType2, eventSource2, transformedEventBody),
		eventshub.StartReceiver,
	))

	f.Setup("install sink2", eventshub.Install(sink2, eventshub.StartReceiver))

	// Install the Triggers with appropriate Sinks and filters
	f.Setup("install trigger1", trigger.Install(
		trigger1,
		trigger.WithBrokerName(brokerName),
		trigger.WithSubscriber(service.AsKReference(sink1), ""),
		trigger.WithFilter(map[string]string{"type": eventType1, "source": eventSource1}),
	))
	f.Setup("trigger1 goes ready", trigger.IsReady(trigger1))

	f.Setup("install trigger2", trigger.Install(
		trigger2,
		trigger.WithBrokerName(brokerName),
		trigger.WithSubscriber(service.AsKReference(sink2), ""),
		trigger.WithFilter(map[string]string{"type": eventType2, "source": eventSource2}),
	))
	f.Setup("trigger2 goes ready", trigger.IsReady(trigger2))

	// Install the Source
	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
	))

	f.Assert("receive long event on sink1 exactly once",
		eventasssert.OnStore(sink1).
			MatchEvent(test.HasData([]byte(eventBody))).
			Exact(1),
	)

	f.Assert("receive long event on sink2 exactly once",
		eventasssert.OnStore(sink2).
			MatchEvent(test.HasData([]byte(transformedEventBody))).
			Exact(1),
	)

	return f
}

type eventTestCase struct {
	Type       string
	Source     string
	Extensions map[string]interface{}
}

func newEventTestCase(tp, source string) eventTestCase {
	return eventTestCase{Type: tp, Source: source}
}

func newEventTestCaseWithExtensions(tp string, source string, extensions map[string]interface{}) eventTestCase {
	return eventTestCase{Type: tp, Source: source, Extensions: extensions}
}

// toEventMatcher converts the test case to the event matcher
func (tc eventTestCase) toEventMatcher() test.EventMatcher {
	var matchers []test.EventMatcher
	if tc.Type == eventingv1.TriggerAnyFilter {
		matchers = append(matchers, test.ContainsAttributes(spec.Type))
	} else {
		matchers = append(matchers, test.HasType(tc.Type))
	}

	if tc.Source == eventingv1.TriggerAnyFilter {
		matchers = append(matchers, test.ContainsAttributes(spec.Source))
	} else {
		matchers = append(matchers, test.HasSource(tc.Source))
	}

	for k, v := range tc.Extensions {
		if v == eventingv1.TriggerAnyFilter {
			matchers = append(matchers, test.ContainsExtensions(k))
		} else {
			matchers = append(matchers, test.HasExtension(k, v))
		}
	}

	return test.AllOf(matchers...)
}

func (tc eventTestCase) toID() string {
	id := fmt.Sprintf("%s-%s", tc.Type, tc.Source)
	for k, v := range tc.Extensions {
		id += fmt.Sprintf("-%s_%s", k, v)
	}
	return id
}
