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
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"

	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/channel"
	"knative.dev/eventing/test/rekt/resources/subscription"
	"knative.dev/eventing/test/rekt/resources/trigger"
	eventingduckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"

	"knative.dev/reconciler-test/pkg/eventshub"
	eventasssert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/resources/svc"
)

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

func BrokerChannelFlowWithTransformation(createSubscriberFn func(ref *eventingduckv1.KReference, uri string) manifest.CfgFn) *feature.Feature {
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
		brokerName,
		trigger.WithFilter(filter1),
		trigger.WithSubscriber(svc.AsKReference(sink1), ""),
	))
	f.Setup("trigger1 goes ready", trigger.IsReady(trigger1))
	// Install the trigger2 point to Broker to filter all the events
	f.Setup("install trigger2", trigger.Install(
		trigger2,
		brokerName,
		trigger.WithFilter(filter2),
		trigger.WithSubscriber(svc.AsKReference(sink2), ""),
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
		createSubscriberFn(svc.AsKReference(sink3), ""),
	))
	f.Setup("subscription is ready", subscription.IsReady(sub))
	f.Setup("channel is ready", channel.IsReady(channelName))

	// Install the trigger3 point to Broker to filter the events after transformation point to channel
	f.Setup("install trigger3", trigger.Install(
		trigger3,
		brokerName,
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
	cfg := []manifest.CfgFn{trigger.WithSubscriber(svc.AsKReference(sink), "")}

	// Install the trigger
	f.Setup("install trigger", trigger.Install(via, brokerName, cfg...))
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
	cfg := []manifest.CfgFn{trigger.WithSubscriber(svc.AsKReference(sink), "")}

	// Install the trigger
	f.Setup("install trigger", trigger.Install(via, brokerName, cfg...))
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
	cfg := []manifest.CfgFn{trigger.WithSubscriber(svc.AsKReference(sink), "")}

	// Install the trigger
	f.Setup("install trigger", trigger.Install(via, brokerName, cfg...))
	f.Setup("trigger goes ready", trigger.IsReady(via))

	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
		eventshub.DropFirstN(dropNum),
	))

	f.Stable("Broker Redelivery failed the first n events").
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

	//Install the broker
	brokerName := feature.MakeRandomK8sName("broker")
	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Requirement("broker is ready", broker.IsReady(brokerName))
	f.Requirement("broker is addressable", broker.IsAddressable(brokerName))

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	// Install the trigger and Point the Trigger subscriber to the sink svc.
	f.Setup("install trigger", trigger.Install(
		triggerName,
		brokerName,
		trigger.WithSubscriber(nil, "http://fake.svc.cluster.local"),
		trigger.WithDeadLetterSink(svc.AsKReference(sink), ""),
	))
	f.Setup("trigger goes ready", trigger.IsReady(triggerName))

	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
	))

	f.Assert("Receives dls extensions when subscriber is unreachable",
		eventasssert.OnStore(sink).
			MatchEvent(
				test.HasExtension("knativeerrordest", "http://fake.svc.cluster.local"),
			).
			AtLeast(1),
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
		brokerName,
		trigger.WithSubscriber(svc.AsKReference(failer), ""),
		trigger.WithDeadLetterSink(svc.AsKReference(sink), ""),
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
			failerAddress, _ := svc.Address(ctx, failer)
			return test.HasExtension("knativeerrordest", failerAddress.String())
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
		brokerName,
		trigger.WithSubscriber(svc.AsKReference(failer), ""),
		trigger.WithDeadLetterSink(svc.AsKReference(sink), ""),
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
			failerAddress, _ := svc.Address(ctx, failer)
			return test.HasExtension("knativeerrordest", failerAddress.String())
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
		brokerName,
		trigger.WithSubscriber(svc.AsKReference(sink), ""),
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
