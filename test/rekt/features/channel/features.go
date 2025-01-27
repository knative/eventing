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

package channel

import (
	"context"
	"encoding/base64"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/network"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"

	"knative.dev/reconciler-test/pkg/eventshub/assert"

	eventasssert "knative.dev/reconciler-test/pkg/eventshub/assert"

	v1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/test/rekt/features"
	"knative.dev/eventing/test/rekt/resources/channel"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/eventing/test/rekt/resources/containersource"
	"knative.dev/eventing/test/rekt/resources/delivery"
	"knative.dev/eventing/test/rekt/resources/subscription"
)

func ChannelChain(length int, createSubscriberFn func(ref *duckv1.KReference, uri string) manifest.CfgFn) *feature.Feature {
	f := feature.NewFeature()

	sink, channel := ChannelChainSetup(f, length, createSubscriberFn)

	ChannelChainAssert(f, sink, channel)

	return f
}

func ChannelChainSetup(f *feature.Feature, length int, createSubscriberFn func(ref *duckv1.KReference, uri string) manifest.CfgFn) (sink string, channel string) {
	sink = feature.MakeRandomK8sName("sink")

	var channels []string
	for i := 0; i < length; i++ {
		name := feature.MakeRandomK8sName(fmt.Sprintf("channel-%04d", i))
		channels = append(channels, name)
		f.Setup("install channel", channel_impl.Install(name))
		f.Setup("channel is ready", channel_impl.IsReady(name))
	}

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	// use the rest for the chain
	for i := 0; i < length; i++ {
		sub := feature.MakeRandomK8sName(fmt.Sprintf("subscription-%04d", i))
		if i == length-1 {
			// install the final connection to the sink
			f.Setup("install sink subscription", subscription.Install(sub,
				subscription.WithChannel(channel_impl.AsRef(channels[i])),
				createSubscriberFn(service.AsKReference(sink), ""),
			))
		} else {
			f.Setup("install subscription", subscription.Install(sub,
				subscription.WithChannel(channel_impl.AsRef(channels[i])),
				createSubscriberFn(channel_impl.AsRef(channels[i+1]), ""),
			))
		}

		f.Setup("subscription is ready", subscription.IsReady(sub))
	}

	return sink, channels[0]
}

func ChannelChainAssert(f *feature.Feature, sink, channel string) {
	cs := feature.MakeRandomK8sName("containersource")
	eventType := feature.MakeRandomK8sName("et")
	args := "--eventType=" + eventType
	f.Requirement("install containersource", containersource.Install(cs,
		containersource.WithSink(channel_impl.AsDestinationRef(channel)),
		containersource.WithArgs(args)))
	f.Requirement("containersource goes ready", containersource.IsReady(cs))

	f.Assert("chained channels relay events", assert.OnStore(sink).MatchEvent(test.HasType(eventType)).AtLeast(1))
}

func DeadLetterSink(createSubscriberFn func(ref *duckv1.KReference, uri string) manifest.CfgFn) *feature.Feature {
	f := feature.NewFeature()
	sink := feature.MakeRandomK8sName("sink")
	failer := feature.MakeRandomK8sName("failer")
	cs := feature.MakeRandomK8sName("containersource")
	name := feature.MakeRandomK8sName("channel")
	sub := feature.MakeRandomK8sName("subscription")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install failing receiver", eventshub.Install(failer, eventshub.StartReceiver, eventshub.DropFirstN(1)))
	f.Setup("install channel", channel_impl.Install(name, delivery.WithDeadLetterSink(service.AsKReference(sink), "")))
	f.Setup("install subscription", subscription.Install(sub,
		subscription.WithChannel(channel_impl.AsRef(name)),
		createSubscriberFn(service.AsKReference(failer), ""),
	))
	f.Setup("channel is ready", channel_impl.IsReady(name))
	f.Setup("subscription is ready", subscription.IsReady(sub))

	f.Requirement("install containersource", containersource.Install(cs, containersource.WithSink(channel_impl.AsDestinationRef(name))))
	f.Requirement("containersource is ready", containersource.IsReady(cs))
	f.Requirement("Channel has dead letter sink uri", channel_impl.HasDeadLetterSinkURI(name, channel_impl.GVR()))

	f.Assert("dls receives events", func(ctx context.Context, t feature.T) {
		assert.OnStore(sink).
			Match(features.HasKnNamespaceHeader(environment.FromContext(ctx).Namespace())).
			MatchEvent(test.HasType("dev.knative.eventing.samples.heartbeat")).
			AtLeast(1)(ctx, t)
	})

	return f
}

func AsyncHandler(createSubscriberFn func(ref *duckv1.KReference, uri string) manifest.CfgFn) *feature.Feature {
	f := feature.NewFeature()
	sink := feature.MakeRandomK8sName("sink")
	source := feature.MakeRandomK8sName("source")
	name := feature.MakeRandomK8sName("channel")
	sub := feature.MakeRandomK8sName("subscription")

	event := test.FullEvent()
	event.SetID(uuid.New().String())

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install channel", channel_impl.Install(name, channel_impl.WithAnnotations(map[string]interface{}{
		v1.AsyncHandlerAnnotation: "true",
	})))
	f.Setup("install subscription", subscription.Install(sub,
		subscription.WithChannel(channel_impl.AsRef(name)),
		createSubscriberFn(service.AsKReference(sink), ""),
	))
	f.Setup("channel is ready", channel_impl.IsReady(name))
	f.Setup("subscription is ready", subscription.IsReady(sub))

	f.Requirement("install source", eventshub.Install(source, eventshub.InputEvent(event), eventshub.StartSenderToResource(channel_impl.GVR(), name)))

	f.Assert("Event sent", assert.OnStore(source).
		MatchSentEvent(test.HasId(event.ID())).
		AtLeast(1),
	)
	f.Assert("sink receives event", assert.OnStore(sink).
		MatchEvent(test.HasId(event.ID())).
		AtLeast(1),
	)

	return f
}

func AsyncHandlerUpdate(createSubscriberFn func(ref *duckv1.KReference, uri string) manifest.CfgFn) *feature.Feature {
	f := feature.NewFeature()
	sink := feature.MakeRandomK8sName("sink")
	source := feature.MakeRandomK8sName("source")
	name := feature.MakeRandomK8sName("channel")
	sub := feature.MakeRandomK8sName("subscription")

	event := test.FullEvent()
	event.SetID(uuid.New().String())

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install channel", channel_impl.Install(name, channel_impl.WithAnnotations(map[string]interface{}{
		v1.AsyncHandlerAnnotation: "true",
	})))
	f.Setup("install subscription", subscription.Install(sub,
		subscription.WithChannel(channel_impl.AsRef(name)),
		createSubscriberFn(service.AsKReference(sink), ""),
	))
	f.Setup("channel is ready", channel_impl.IsReady(name))
	f.Setup("subscription is ready", subscription.IsReady(sub))

	f.Requirement("update channel async handler", func(ctx context.Context, t feature.T) {
		dc := Client(ctx)

		imc, err := dc.ChannelImpl.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Failed to retrieve InMemoryChannel: %v", err)
		}
		// swap and update it to false
		imc.SetAnnotations(map[string]string{
			v1.AsyncHandlerAnnotation: "true",
		})
		if _, err := dc.ChannelImpl.Update(ctx, imc, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("Failed to update async handler annotation: %v", err)
		}
	})

	f.Requirement("channel is ready", channel_impl.IsReady(name))
	f.Requirement("install source", eventshub.Install(source, eventshub.InputEvent(event), eventshub.StartSenderToResource(channel_impl.GVR(), name)))

	f.Assert("Event sent", assert.OnStore(source).
		MatchSentEvent(test.HasId(event.ID())).
		AtLeast(1),
	)
	f.Assert("sink receives event", assert.OnStore(sink).
		MatchEvent(test.HasId(event.ID())).
		AtLeast(1),
	)

	return f
}

func DeadLetterSinkGenericChannel(createSubscriberFn func(ref *duckv1.KReference, uri string) manifest.CfgFn) *feature.Feature {
	f := feature.NewFeature()
	sink := feature.MakeRandomK8sName("sink")
	failer := feature.MakeRandomK8sName("failer")
	cs := feature.MakeRandomK8sName("containersource")
	name := feature.MakeRandomK8sName("channel")
	sub := feature.MakeRandomK8sName("subscription")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install failing receiver", eventshub.Install(failer, eventshub.StartReceiver, eventshub.DropFirstN(1)))
	f.Setup("install channel", channel.Install(name,
		channel.WithTemplate(),
		delivery.WithDeadLetterSink(service.AsKReference(sink), "")),
	)
	f.Setup("install subscription", subscription.Install(sub,
		subscription.WithChannel(channel.AsRef(name)),
		createSubscriberFn(service.AsKReference(failer), ""),
	))
	f.Setup("channel is ready", channel.IsReady(name))
	f.Setup("subscription is ready", subscription.IsReady(sub))

	f.Requirement("install containersource", containersource.Install(cs, containersource.WithSink(channel_impl.AsDestinationRef(name))))
	f.Requirement("containersource is ready", containersource.IsReady(cs))
	f.Requirement("Channel has dead letter sink uri", channel_impl.HasDeadLetterSinkURI(name, channel.GVR()))

	f.Assert("dls receives events", func(ctx context.Context, t feature.T) {
		assert.OnStore(sink).
			Match(features.HasKnNamespaceHeader(environment.FromContext(ctx).Namespace())).
			MatchEvent(test.HasType("dev.knative.eventing.samples.heartbeat")).
			AtLeast(1)(ctx, t)
	})

	return f
}

func AsDeadLetterSink(createSubscriberFn func(ref *duckv1.KReference, uri string) manifest.CfgFn) *feature.Feature {
	f := feature.NewFeatureNamed("As dead letter sink")

	cs := feature.MakeRandomK8sName("containersource")

	name := feature.MakeRandomK8sName("channel")
	dls := feature.MakeRandomK8sName("dls-channel")

	failer := feature.MakeRandomK8sName("failer")
	sink := feature.MakeRandomK8sName("sink")

	f.Setup("install containersource", containersource.Install(cs, containersource.WithSink(channel_impl.AsDestinationRef(name))))

	f.Setup("install channel", channel.Install(name,
		channel.WithTemplate(),
		delivery.WithDeadLetterSink(channel.AsRef(dls), "")),
	)
	f.Setup("install subscription", subscription.Install(feature.MakeRandomK8sName("subscription"),
		subscription.WithChannel(channel.AsRef(name)),
		createSubscriberFn(service.AsKReference(failer), ""),
	))

	f.Setup("install DLS channel", channel.Install(dls,
		channel.WithTemplate(),
	))
	f.Setup("install DLS subscription", subscription.Install(feature.MakeRandomK8sName("dls-subscription"),
		subscription.WithChannel(channel.AsRef(dls)),
		createSubscriberFn(service.AsKReference(sink), ""),
	))

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install failing receiver", eventshub.Install(failer, eventshub.StartReceiver, eventshub.DropFirstN(10)))

	f.Setup("channel is ready", channel.IsReady(name))
	f.Setup("channel is ready", channel.IsReady(dls))
	f.Setup("containersource is ready", containersource.IsReady(cs))

	f.Requirement("Channel has dead letter sink uri", channel_impl.HasDeadLetterSinkURI(name, channel.GVR()))

	f.Assert("dls receives events", assert.OnStore(sink).
		MatchEvent(test.HasType("dev.knative.eventing.samples.heartbeat")).
		AtLeast(1),
	)

	return f
}

func EventTransformation() *feature.Feature {
	f := feature.NewFeature()
	channel1 := feature.MakeRandomK8sName("channel 1")
	channel2 := feature.MakeRandomK8sName("channel 2")
	subscription1 := feature.MakeRandomK8sName("subscription 1")
	subscription2 := feature.MakeRandomK8sName("subscription 2")
	prober := eventshub.NewProber()
	prober.SetTargetResource(channel_impl.GVR(), channel1)
	prober.SenderFullEvents(3)

	f.Setup("install sink", prober.ReceiverInstall("sink"))
	f.Setup("install transform service", prober.ReceiverInstall("transform", eventshub.ReplyWithTransformedEvent("transformed", "transformer", "")))
	f.Setup("install channel 1", channel_impl.Install(channel1))
	f.Setup("install channel 2", channel_impl.Install(channel2))
	f.Setup("install subscription 1", subscription.Install(subscription1,
		subscription.WithChannel(channel_impl.AsRef(channel1)),
		subscription.WithSubscriber(prober.AsKReference("transform"), "", ""),
		subscription.WithReply(channel_impl.AsRef(channel2), ""),
	))
	f.Setup("install subscription 2", subscription.Install(subscription2,
		subscription.WithChannel(channel_impl.AsRef(channel2)),
		subscription.WithSubscriber(prober.AsKReference("sink"), "", ""),
	))
	f.Setup("subscription 1 is ready", subscription.IsReady(subscription1))
	f.Setup("subscription 2 is ready", subscription.IsReady(subscription2))
	f.Setup("channel 1 is ready", channel_impl.IsReady(channel1))
	f.Setup("channel 2 is ready", channel_impl.IsReady(channel2))

	f.Requirement("install source", prober.SenderInstall("source"))

	f.Assert("sink receives events", prober.AssertReceivedAll("source", "sink"))
	f.Assert("events have passed through transform service", func(ctx context.Context, t feature.T) {
		events := prober.ReceivedBy(ctx, "sink")
		if len(events) != 3 {
			t.Errorf("expected 3 events, got %d", len(events))
		}
		for _, e := range events {
			if e.Event.Type() != "transformed" {
				t.Errorf(`expected event type to be "transformed", got %q`, e.Event.Type())
			}
		}
	})
	return f
}

func SingleEventWithEncoding(encoding binding.Encoding) *feature.Feature {
	f := feature.NewFeature()
	channel := feature.MakeRandomK8sName("channel")
	sub := feature.MakeRandomK8sName("subscription")
	prober := eventshub.NewProber()
	prober.SetTargetResource(channel_impl.GVR(), channel)

	event := cloudevents.NewEvent()
	event.SetID(feature.MakeRandomK8sName("test"))
	event.SetType("myevent")
	event.SetSource("http://sender.svc/")
	prober.ExpectEvents([]string{event.ID()})

	f.Setup("install sink", prober.ReceiverInstall("sink"))
	f.Setup("install channel", channel_impl.Install(channel))
	f.Setup("install subscription", subscription.Install(sub,
		subscription.WithChannel(channel_impl.AsRef(channel)),
		subscription.WithSubscriber(prober.AsKReference("sink"), "", ""),
	))

	f.Setup("subscription is ready", subscription.IsReady(sub))
	f.Setup("channel is ready", channel_impl.IsReady(channel))

	f.Requirement("install source", prober.SenderInstall("source", eventshub.InputEventWithEncoding(event, encoding)))

	f.Assert("sink receives events", prober.AssertReceivedAll("source", "sink"))

	return f
}

func ChannelPreferHeaderCheck(createSubscriberFn func(ref *duckv1.KReference, uri string) manifest.CfgFn) *feature.Feature {
	f := feature.NewFeatureNamed("Channel PreferHeader Check")

	channelName := feature.MakeRandomK8sName("channel")
	sub := feature.MakeRandomK8sName("subscription")
	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")

	eventSource := "source1"
	eventType := "type1"
	eventBody := `{"msg":"test msg"}`
	event := cloudevents.NewEvent()
	event.SetID(uuid.New().String())
	event.SetType(eventType)
	event.SetSource(eventSource)
	event.SetData(cloudevents.ApplicationJSON, []byte(eventBody))

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install channel", channel.Install(channelName,
		channel.WithTemplate(),
	))
	f.Setup("install subscription", subscription.Install(sub,
		subscription.WithChannel(channel.AsRef(channelName)),
		createSubscriberFn(service.AsKReference(sink), ""),
	))

	f.Setup("subscription is ready", subscription.IsReady(sub))
	f.Setup("channel is ready", channel.IsReady(channelName))

	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResource(channel.GVR(), channelName),
		eventshub.InputEvent(event),
	))

	f.Stable("test message without explicit prefer header should have the header").
		Must("delivers events", func(ctx context.Context, t feature.T) {
			eventasssert.OnStore(sink).Match(
				features.HasKnNamespaceHeader(environment.FromContext(ctx).Namespace()),
				eventasssert.HasAdditionalHeader("Prefer", "reply"),
			).AtLeast(1)(ctx, t)
		})

	return f
}

func ChannelDeadLetterSinkExtensions(createSubscriberFn func(ref *duckv1.KReference, uri string) manifest.CfgFn) *feature.FeatureSet {
	fs := &feature.FeatureSet{
		Name: "Knative Channel - DeadLetterSink - with Extensions",
		Features: []*feature.Feature{
			channelSubscriberUnreachable(createSubscriberFn),
			channelSubscriberReturnedErrorNoData(createSubscriberFn),
			channelSubscriberReturnedErrorWithData(createSubscriberFn),
		},
	}
	return fs
}

func channelSubscriberUnreachable(createSubscriberFn func(ref *duckv1.KReference, uri string) manifest.CfgFn) *feature.Feature {
	f := feature.NewFeature()
	sink := feature.MakeRandomK8sName("sink")

	sourceName := feature.MakeRandomK8sName("source")
	channelName := feature.MakeRandomK8sName("channel")
	sub := feature.MakeRandomK8sName("subscription")

	subscriberUri := fmt.Sprintf("http://fake.svc.%s", network.GetClusterDomainName())

	ev := test.FullEvent()

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	f.Setup("install channel", channel_impl.Install(channelName, delivery.WithDeadLetterSink(service.AsKReference(sink), "")))

	f.Setup("install subscription", subscription.Install(sub,
		subscription.WithChannel(channel_impl.AsRef(channelName)),
		createSubscriberFn(nil, subscriberUri),
	))
	f.Setup("channel is ready", channel_impl.IsReady(channelName))
	f.Setup("channel is addressable", channel_impl.IsAddressable(channelName))
	f.Setup("subscription is ready", subscription.IsReady(sub))
	f.Setup("channel has dead letter sink uri", channel_impl.HasDeadLetterSinkURI(channelName, channel_impl.GVR()))

	f.Requirement("install source", eventshub.Install(
		sourceName,
		eventshub.StartSenderToResource(channel_impl.GVR(), channelName),
		eventshub.InputEvent(ev),
	))

	f.Assert("Receives dls extensions when subscriber is unreachable", eventasssert.OnStore(sink).
		MatchEvent(
			test.HasExtension("knativeerrordest", subscriberUri)).
		AtLeast(1),
	)

	return f
}

func channelSubscriberReturnedErrorNoData(createSubscriberFn func(ref *duckv1.KReference, uri string) manifest.CfgFn) *feature.Feature {
	f := feature.NewFeature()
	sink := feature.MakeRandomK8sName("sink")

	sourceName := feature.MakeRandomK8sName("source")
	failer := feature.MakeRandomK8sName("failerWitdata")
	channelName := feature.MakeRandomK8sName("channel")
	sub := feature.MakeRandomK8sName("subscription")

	ev := test.FullEvent()

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	f.Setup("install failing receiver", eventshub.Install(failer,
		eventshub.StartReceiver,
		eventshub.DropFirstN(1),
		eventshub.DropEventsResponseCode(422),
	))
	f.Setup("install channel", channel_impl.Install(channelName, delivery.WithDeadLetterSink(service.AsKReference(sink), "")))

	f.Setup("install subscription", subscription.Install(sub,
		subscription.WithChannel(channel_impl.AsRef(channelName)),
		createSubscriberFn(service.AsKReference(failer), ""),
	))
	f.Setup("channel is ready", channel_impl.IsReady(channelName))
	f.Setup("channel is addressable", channel_impl.IsAddressable(channelName))
	f.Setup("subscription is ready", subscription.IsReady(sub))
	f.Setup("channel has dead letter sink uri", channel_impl.HasDeadLetterSinkURI(channelName, channel_impl.GVR()))

	f.Requirement("install source", eventshub.Install(
		sourceName,
		eventshub.StartSenderToResource(channel_impl.GVR(), channelName),
		eventshub.InputEvent(ev),
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

func channelSubscriberReturnedErrorWithData(createSubscriberFn func(ref *duckv1.KReference, uri string) manifest.CfgFn) *feature.Feature {
	f := feature.NewFeature()
	sink := feature.MakeRandomK8sName("sink")

	sourceName := feature.MakeRandomK8sName("source")
	failer := feature.MakeRandomK8sName("failerWitdata")
	channelName := feature.MakeRandomK8sName("channel")
	sub := feature.MakeRandomK8sName("subscription")

	ev := test.FullEvent()

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	errorData := "<!doctype html>\n<html>\n<head>\n    <title>Error Page(tm)</title>\n</head>\n<body>\n<p>Quoth the server, 404!\n</body></html>"
	f.Setup("install failing receiver", eventshub.Install(failer,
		eventshub.StartReceiver,
		eventshub.DropFirstN(1),
		eventshub.DropEventsResponseCode(422),
		eventshub.DropEventsResponseBody(errorData),
	))
	f.Setup("install channel", channel_impl.Install(channelName, delivery.WithDeadLetterSink(service.AsKReference(sink), "")))
	f.Setup("install subscription", subscription.Install(sub,
		subscription.WithChannel(channel_impl.AsRef(channelName)),
		createSubscriberFn(service.AsKReference(failer), ""),
	))
	f.Setup("channel is ready", channel_impl.IsReady(channelName))
	f.Setup("channel is addressable", channel_impl.IsAddressable(channelName))
	f.Setup("subscription is ready", subscription.IsReady(sub))
	f.Setup("channel has dead letter sink uri", channel_impl.HasDeadLetterSinkURI(channelName, channel_impl.GVR()))

	f.Requirement("install source", eventshub.Install(
		sourceName,
		eventshub.StartSenderToResource(channel_impl.GVR(), channelName),
		eventshub.InputEvent(ev),
	))

	f.Assert("Receives dls extensions with errordata Base64encoding", assertEnhancedWithKnativeErrorExtensions(
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
			assert.MatchKind(eventshub.EventReceived),
			assert.MatchEvent(matchers...),
		)
	}
}
