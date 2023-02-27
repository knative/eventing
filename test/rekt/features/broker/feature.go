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
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"

	eventingv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/trigger"
	"knative.dev/pkg/ptr"

	"knative.dev/reconciler-test/pkg/eventshub"
	eventasssert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/resources/svc"
)

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

	exp := eventingv1.BackoffPolicyLinear
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
	exp := eventingv1.BackoffPolicyLinear
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
		brokerName,
		trigger.WithSubscriber(svc.AsKReference(sink1), ""),
		trigger.WithFilter(map[string]string{"type": eventType1, "source": eventSource1}),
	))
	f.Setup("trigger1 goes ready", trigger.IsReady(trigger1))

	f.Setup("install trigger2", trigger.Install(
		trigger2,
		brokerName,
		trigger.WithSubscriber(svc.AsKReference(sink2), ""),
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
