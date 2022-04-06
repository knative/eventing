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

package trigger

import (
	"context"
	"fmt"

	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"

	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/delivery"
	"knative.dev/eventing/test/rekt/resources/eventlibrary"
	"knative.dev/eventing/test/rekt/resources/trigger"
)

// SourceToTriggerSinkWithDLS tests to see if a Ready Trigger with a DLS defined send
// failing events to it's DLS.
//
// source ---> broker --[trigger]--> bad uri
//                          |
//                          +--[DLS]--> sink
//
func SourceToTriggerSinkWithDLS() *feature.Feature {
	f := feature.NewFeatureNamed("Trigger with DLS")

	triggerName := feature.MakeRandomK8sName("trigger")
	brokerName := feature.MakeRandomK8sName("broker")
	triggerSinkName := feature.MakeRandomK8sName("trigger-sink")

	prober := eventshub.NewProber()
	prober.SetTargetResource(broker.GVR(), brokerName)

	lib := feature.MakeRandomK8sName("lib")
	f.Setup("install events", eventlibrary.Install(lib))
	f.Setup("event cache is ready", eventlibrary.IsReady(lib))
	f.Setup("use events cache", prober.SenderEventsFromSVC(lib, "events/three.ce"))
	if err := prober.ExpectYAMLEvents(eventlibrary.PathFor("events/three.ce")); err != nil {
		panic(fmt.Errorf("can not find event files: %s", err))
	}

	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))

	// Setup Probes
	f.Setup("install recorder", prober.ReceiverInstall(triggerSinkName))

	// Setup trigger
	f.Setup("install trigger", trigger.Install(
		triggerName,
		brokerName,
		trigger.WithSubscriber(nil, "bad://uri"),
		delivery.WithDeadLetterSink(prober.AsKReference(triggerSinkName), "")))

	// Resources ready.
	f.Setup("trigger goes ready", trigger.IsReady(triggerName))

	// Install sender.
	f.Setup("install source", prober.SenderInstall("source"))

	// After we have finished sending.
	f.Requirement("sender is finished", prober.SenderDone("source"))

	// Assert events ended up where we expected.
	f.Stable("trigger with DLS").
		Must("accepted all events", prober.AssertSentAll("source")).
		Must("deliver event to DLS", prober.AssertReceivedAll("source", triggerSinkName))

	return f
}

// SourceToTriggerSinkWithDLSDontUseBrokers tests to see if a Ready Trigger sends
// failing events to it's DLS even when it's corresponding Ready Broker also have a DLS defined.
//
// source ---> broker --[trigger]--> bad uri
//               |          |
//               +--[DLS]   +--[DLS]--> sink
//
func SourceToTriggerSinkWithDLSDontUseBrokers() *feature.Feature {
	f := feature.NewFeatureNamed("When Trigger DLS is defined, Broker DLS is ignored")

	triggerName := feature.MakeRandomK8sName("trigger")
	brokerName := feature.MakeRandomK8sName("broker")
	triggerSinkName := feature.MakeRandomK8sName("trigger-sink")
	brokerSinkName := feature.MakeRandomK8sName("broker-sink")

	prober := eventshub.NewProber()
	prober.SetTargetResource(broker.GVR(), brokerName)

	lib := feature.MakeRandomK8sName("lib")
	f.Setup("install events", eventlibrary.Install(lib))
	f.Setup("event cache is ready", eventlibrary.IsReady(lib))
	f.Setup("use events cache", prober.SenderEventsFromSVC(lib, "events/three.ce"))
	if err := prober.ExpectYAMLEvents(eventlibrary.PathFor("events/three.ce")); err != nil {
		panic(fmt.Errorf("can not find event files: %s", err))
	}

	// Setup Probes
	f.Setup("install trigger recorder", prober.ReceiverInstall(triggerSinkName))
	f.Setup("install brokers recorder", prober.ReceiverInstall(brokerSinkName))

	// Setup topology
	brokerConfig := append(
		broker.WithEnvConfig(),
		delivery.WithDeadLetterSink(prober.AsKReference(brokerSinkName), ""))
	f.Setup("install broker with DLS", broker.Install(
		brokerName,
		brokerConfig...,
	))

	f.Setup("install trigger", trigger.Install(
		triggerName,
		brokerName,
		trigger.WithSubscriber(nil, "bad://uri"),
		delivery.WithDeadLetterSink(prober.AsKReference(triggerSinkName), "")))

	// Resources ready.
	f.Setup("trigger goes ready", trigger.IsReady(triggerName))

	// Install events after topology is ready.
	f.Setup("install source", prober.SenderInstall("source"))

	// After we have finished sending.
	f.Requirement("sender is finished", prober.SenderDone("source"))

	// Assert events ended up where we expected.
	f.Stable("trigger with a valid DLS ref").
		Must("accept all events", prober.AssertSentAll("source")).
		Must("deliver events to trigger DLS", prober.AssertReceivedAll("source", triggerSinkName)).
		Must("not deliver events to its broker DLS", noEventsToDLS(prober, brokerSinkName))

	return f
}

// source ---> broker +--[trigger<via1>]--> bad uri
//                |   |
//                |   +--[trigger<vai2>]--> sink
//                |
//                +--[DLQ]--> dlq
//
func BadTriggerDoesNotAffectOkTrigger() *feature.Feature {
	f := feature.NewFeatureNamed("Bad Trigger does not affect good Trigger")

	prober := eventshub.NewProber()
	brokerName := feature.MakeRandomK8sName("broker")
	via1 := feature.MakeRandomK8sName("via")
	via2 := feature.MakeRandomK8sName("via")
	dlq := feature.MakeRandomK8sName("dlq")
	sink := feature.MakeRandomK8sName("sink")
	source := feature.MakeRandomK8sName("source")

	lib := feature.MakeRandomK8sName("lib")
	f.Setup("install events", eventlibrary.Install(lib))
	f.Setup("event cache is ready", eventlibrary.IsReady(lib))
	f.Setup("use events cache", prober.SenderEventsFromSVC(lib, "events/three.ce"))
	if err := prober.ExpectYAMLEvents(eventlibrary.PathFor("events/three.ce")); err != nil {
		panic(fmt.Errorf("can not find event files: %s", err))
	}

	// Setup Probes
	f.Setup("install dlq", prober.ReceiverInstall(dlq))
	f.Setup("install sink2", prober.ReceiverInstall(sink))

	// Setup data plane
	brokerConfig := append(broker.WithEnvConfig(), delivery.WithDeadLetterSink(prober.AsKReference(dlq), ""))
	f.Setup("install broker", broker.Install(brokerName, brokerConfig...))
	// Block till broker is ready
	f.Setup("Broker is ready", broker.IsReady(brokerName))
	prober.SetTargetResource(broker.GVR(), brokerName)

	f.Setup("install trigger via1", trigger.Install(via1, brokerName, trigger.WithSubscriber(nil, "bad://uri")))
	f.Setup("install trigger via2", trigger.Install(via2, brokerName, trigger.WithSubscriber(prober.AsKReference(sink), "")))

	// Resources ready.
	f.Setup("trigger1 goes ready", trigger.IsReady(via1))
	f.Setup("trigger2 goes ready", trigger.IsReady(via2))

	// Install events after data plane is ready.
	f.Requirement("install source", prober.SenderInstall(source))

	// After we have finished sending.
	f.Requirement("sender is finished", prober.SenderDone(source))
	f.Requirement("receiver 1 is finished", prober.ReceiverDone(source, dlq))
	f.Requirement("receiver 2 is finished", prober.ReceiverDone(source, sink))

	// Assert events ended up where we expected.
	f.Stable("broker with DLQ").
		Must("accepted all events", prober.AssertSentAll(source)).
		Must("deliver event to DLQ (via1)", prober.AssertReceivedAll(source, dlq)).
		Must("deliver event to sink (via2)", prober.AssertReceivedAll(source, sink))

	return f
}

func noEventsToDLS(prober *eventshub.EventProber, sinkName string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		if len(prober.ReceivedBy(ctx, sinkName)) == 0 {
			t.Log("no events were sent to %s DLS", sinkName)
		} else {
			t.Errorf("events were received by %s DLS", sinkName)
		}
	}
}
