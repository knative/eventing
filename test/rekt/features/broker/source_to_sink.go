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

package broker

import (
	"time"

	"github.com/google/uuid"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"

	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/delivery"
	"knative.dev/eventing/test/rekt/resources/trigger"

	. "github.com/cloudevents/sdk-go/v2/test"
	. "knative.dev/reconciler-test/pkg/eventshub/assert"
)

// SourceToSink tests to see if a Ready Broker acts as middleware.
// LoadGenerator --> in [Broker] out --> Recorder
func SourceToSink(brokerName string) *feature.Feature {
	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	via := feature.MakeRandomK8sName("via")
	event := FullEvent()

	f := new(feature.Feature)

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	// Point the Trigger subscriber to the sink svc.
	cfg := []manifest.CfgFn{trigger.WithSubscriber(service.AsKReference(sink), ""), trigger.WithBrokerName(brokerName)}

	// Install the trigger
	f.Setup("install trigger", trigger.Install(via, cfg...))

	f.Setup("trigger goes ready", trigger.IsReady(via))

	f.Requirement("install source", eventshub.Install(source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
	))

	f.Stable("broker as middleware").
		Must("deliver an event",
			OnStore(sink).MatchEvent(HasId(event.ID())).Exact(1))

	return f
}

// SourceToSinkWithDLQ tests to see if a Trigger with no DLQ and a bad Sink, subscribed
// to a Ready Broker, forward unsent events to it's Broker's DLQ
//
// source ---> broker<Via> --[trigger]--> bad uri
//
//	|
//	+--[DLQ]--> dlq
func SourceToSinkWithDLQ() *feature.Feature {
	f := feature.NewFeature()

	brokerName := feature.MakeRandomK8sName("broker")
	dls := feature.MakeRandomK8sName("dls")
	triggerName := feature.MakeRandomK8sName("trigger")
	source := feature.MakeRandomK8sName("source")

	f.Setup("install dead letter sink service", eventshub.Install(dls, eventshub.StartReceiver))

	brokerConfig := append(broker.WithEnvConfig(), delivery.WithDeadLetterSink(service.AsKReference(dls), ""))
	f.Setup("install broker", broker.Install(brokerName, brokerConfig...))
	f.Setup("Broker is ready", broker.IsReady(brokerName))
	f.Setup("install trigger", trigger.Install(triggerName, trigger.WithBrokerName(brokerName), trigger.WithSubscriber(nil, "bad://uri")))
	f.Setup("trigger is ready", trigger.IsReady(triggerName))

	ce := FullEvent()
	ce.SetID(uuid.New().String())

	// Send events after data plane is ready.
	f.Requirement("install source", eventshub.Install(source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(ce),
	))

	// Assert events ended up where we expected.
	f.Stable("broker with DLS").
		Must("deliver event to DLQ", OnStore(dls).MatchEvent(HasId(ce.ID())).AtLeast(1))

	return f
}

// SourceToSinkWithFlakyDLQ tests to see if a Ready Broker acts as middleware.
//
// source ---> broker --[trigger]--> flake 1/3 --> recorder
//
//	|
//	+--[DLQ]--> recorder
func SourceToSinkWithFlakyDLQ(brokerName string) *feature.Feature {
	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	dlq := feature.MakeRandomK8sName("dlq")
	via := feature.MakeRandomK8sName("via")

	f := feature.NewFeatureNamed("Source to sink with flaky DLS")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	f.Setup("install dlq", eventshub.Install(dlq, eventshub.StartReceiver))
	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver, eventshub.DropFirstN(2)))
	f.Setup("update broker with DLQ", broker.Install(brokerName, broker.WithDeadLetterSink(service.AsKReference(dlq), "")))
	f.Setup("install trigger", trigger.Install(via, trigger.WithBrokerName(brokerName), trigger.WithSubscriber(service.AsKReference(sink), "")))
	f.Setup("trigger goes ready", trigger.IsReady(via))
	f.Setup("broker goes ready", broker.IsReady(via))

	f.Requirement("install source", eventshub.Install(source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.SendMultipleEvents(3, time.Millisecond),
		eventshub.EnableIncrementalId,
	))

	f.Stable("broker with DLS").
		Must("deliver event flaky sent to DLQ event[0]",
			OnStore(dlq).MatchEvent(HasId("1")).Exact(1)).
		Must("deliver event flaky sent to DLQ event[1]",
			OnStore(dlq).MatchEvent(HasId("2")).Exact(1)).
		Must("deliver event sink receiver got event[2]",
			OnStore(sink).MatchEvent(HasId("3")).Exact(1))

	return f
}
