/*
Copyright 2023 The Knative Authors
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

package eventtype_autocreate

import (
	"context"

	"github.com/cloudevents/sdk-go/v2/test"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"k8s.io/apimachinery/pkg/util/sets"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/eventing/test/rekt/resources/containersource"
	"knative.dev/eventing/test/rekt/resources/eventtype"
	"knative.dev/eventing/test/rekt/resources/pingsource"
	"knative.dev/eventing/test/rekt/resources/subscription"
	"knative.dev/eventing/test/rekt/resources/trigger"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"
)

func AutoCreateEventTypesOnIMC() *feature.Feature {
	f := feature.NewFeature()

	event := cetest.FullEvent()
	event.SetType("test.imc.custom.event.type")

	sender := feature.MakeRandomK8sName("sender")
	sub := feature.MakeRandomK8sName("subscription")
	channelName := feature.MakeRandomK8sName("channel")
	sink := feature.MakeRandomK8sName("sink")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install channel", channel_impl.Install(channelName))
	f.Setup("install subscription", subscription.Install(sub,
		subscription.WithChannel(channel_impl.AsRef(channelName)),
		subscription.WithSubscriber(service.AsKReference(sink), "", ""),
	))

	f.Setup("subscription is ready", subscription.IsReady(sub))
	f.Setup("channel is ready", channel_impl.IsReady(channelName))

	f.Requirement("install event sender", eventshub.Install(sender,
		eventshub.StartSenderToResource(channel_impl.GVR(), channelName),
		eventshub.InputEvent(event),
	))

	expectedTypes := sets.New(event.Type())

	f.Alpha("imc").
		Must("deliver events to subscriber", assert.OnStore(sink).MatchEvent(cetest.HasId(event.ID())).AtLeast(1)).
		Must("create event type", eventtype.WaitForEventType(eventtype.AssertPresent(expectedTypes)))

	return f
}

func AutoCreateEventTypesOnBroker(brokerName string) *feature.Feature {
	f := feature.NewFeature()

	event := cetest.FullEvent()
	event.SetType("test.broker.custom.event.type")

	sender := feature.MakeRandomK8sName("sender")
	triggerName := feature.MakeRandomK8sName("trigger")
	sink := feature.MakeRandomK8sName("sink")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install subscription", trigger.Install(triggerName, brokerName, trigger.WithSubscriber(service.AsKReference(sink), "")))

	f.Setup("trigger is ready", trigger.IsReady(triggerName))
	f.Setup("broker is addressable", k8s.IsAddressable(broker.GVR(), brokerName))

	f.Requirement("install event sender", eventshub.Install(sender,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
	))

	expectedTypes := sets.New(event.Type())

	f.Alpha("broker").
		Must("deliver events to subscriber", assert.OnStore(sink).MatchEvent(cetest.HasId(event.ID())).AtLeast(1)).
		Must("create event type", eventtype.WaitForEventType(eventtype.AssertExactPresent(expectedTypes)))

	return f
}

func AutoCreateEventTypeEventsFromPingSource() *feature.Feature {
	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	via := feature.MakeRandomK8sName("via")

	f := new(feature.Feature)

	//Install the broker
	brokerName := feature.MakeRandomK8sName("broker")
	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("broker is ready", broker.IsReady(brokerName))
	f.Setup("broker is addressable", broker.IsAddressable(brokerName))
	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install trigger", trigger.Install(via, brokerName, trigger.WithSubscriber(service.AsKReference(sink), "")))
	f.Setup("trigger goes ready", trigger.IsReady(via))

	f.Requirement("install pingsource", func(ctx context.Context, t feature.T) {
		brokeruri, err := broker.Address(ctx, brokerName)
		if err != nil {
			t.Error("failed to get address of broker", err)
		}
		cfg := []manifest.CfgFn{
			pingsource.WithSink(&duckv1.Destination{URI: brokeruri.URL, CACerts: brokeruri.CACerts}),
			pingsource.WithData("text/plain", "hello, world!"),
		}
		pingsource.Install(source, cfg...)(ctx, t)
	})
	f.Requirement("PingSource goes ready", pingsource.IsReady(source))

	expectedCeTypes := sets.New(sourcesv1.PingSourceEventType)

	f.Stable("pingsource as event source").
		Must("delivers events on broker with URI", assert.OnStore(sink).MatchEvent(
			test.HasType(sourcesv1.PingSourceEventType)).AtLeast(1)).
		Must("PingSource test eventtypes match", eventtype.WaitForEventType(
			eventtype.AssertPresent(expectedCeTypes)))

	return f
}

func AutoCreateEventTypesOnContainerSource() *feature.Feature {
	f := feature.NewFeature()

	event := cetest.FullEvent()
	event.SetType("test.containersource.custom.event.type")

	sourceName := feature.MakeRandomK8sName("containersource")
	sink := feature.MakeRandomK8sName("sink")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	destination := &duckv1.Destination{
		Ref: service.AsKReference(sink),
	}
	f.Setup("install containersource", containersource.Install(sourceName, containersource.WithSink(destination)))

	f.Setup("containersource is ready", containersource.IsReady(sourceName))

	expectedTypes := sets.New(event.Type())

	f.Stable("containersource").
		Must("delivers events to subscriber", assert.OnStore(sink).MatchEvent(cetest.HasId(event.ID())).AtLeast(1)).
		Must("create event type", eventtype.WaitForEventType(eventtype.AssertExactPresent(expectedTypes)))

	return f
}
