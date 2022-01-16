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
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/eventing/test/rekt/resources/containersource"
	"knative.dev/eventing/test/rekt/resources/delivery"
	"knative.dev/eventing/test/rekt/resources/eventlibrary"
	"knative.dev/eventing/test/rekt/resources/pingsource"
	"knative.dev/eventing/test/rekt/resources/source"
	"knative.dev/eventing/test/rekt/resources/subscription"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/resources/svc"
)

func ChannelChain(length int, createSubscriberFn func(ref *duckv1.KReference, uri string) manifest.CfgFn) *feature.Feature {
	f := feature.NewFeature()
	sink := feature.MakeRandomK8sName("sink")
	cs := feature.MakeRandomK8sName("containersource")

	var channels []string
	for i := 0; i < length; i++ {
		name := feature.MakeRandomK8sName(fmt.Sprintf("channel-%04d", i))
		channels = append(channels, name)
		f.Setup("install channel", channel_impl.Install(name))
		f.Requirement("channel is ready", channel_impl.IsReady(name))
	}

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	// attach the first channel to the source
	f.Setup("install containersource", containersource.Install(cs, pingsource.WithSink(channel_impl.AsRef(channels[0]), "")))

	// use the rest for the chain
	for i := 0; i < length; i++ {
		sub := feature.MakeRandomK8sName(fmt.Sprintf("subscription-%04d", i))
		if i == length-1 {
			// install the final connection to the sink
			f.Setup("install sink subscription", subscription.Install(sub,
				subscription.WithChannel(channel_impl.AsRef(channels[i])),
				createSubscriberFn(svc.AsKReference(sink), ""),
			))
		} else {
			f.Setup("install subscription", subscription.Install(sub,
				subscription.WithChannel(channel_impl.AsRef(channels[i])),
				createSubscriberFn(channel_impl.AsRef(channels[i+1]), ""),
			))
		}
	}
	f.Requirement("containersource goes ready", containersource.IsReady(cs))

	f.Assert("chained channels relay events", assert.OnStore(sink).MatchEvent(test.HasType("dev.knative.eventing.samples.heartbeat")).AtLeast(1))

	return f
}

func DeadLetterSink(createSubscriberFn func(ref *duckv1.KReference, uri string) manifest.CfgFn) *feature.Feature {
	f := feature.NewFeature()
	sink := feature.MakeRandomK8sName("sink")
	failer := feature.MakeK8sNamePrefix("failer")
	cs := feature.MakeRandomK8sName("containersource")
	name := feature.MakeRandomK8sName("channel")

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install failing receiver", eventshub.Install(failer, eventshub.StartReceiver, eventshub.DropFirstN(1)))
	f.Setup("install channel", channel_impl.Install(name, delivery.WithDeadLetterSink(svc.AsKReference(sink), "")))
	f.Setup("install containersource", containersource.Install(cs, source.WithSink(channel_impl.AsRef(name), "")))
	f.Setup("install subscription", subscription.Install(feature.MakeRandomK8sName("subscription"),
		subscription.WithChannel(channel_impl.AsRef(name)),
		createSubscriberFn(svc.AsKReference(failer), ""),
	))

	f.Requirement("channel is ready", channel_impl.IsReady(name))
	f.Requirement("containersource is ready", containersource.IsReady(cs))

	f.Assert("dls receives events", assert.OnStore(sink).MatchEvent(test.HasType("dev.knative.eventing.samples.heartbeat")).AtLeast(1))

	return f
}

func EventTransformation() *feature.Feature {
	f := feature.NewFeature()
	lib := feature.MakeRandomK8sName("lib")
	channel1 := feature.MakeRandomK8sName("channel 1")
	channel2 := feature.MakeRandomK8sName("channel 2")
	subscription1 := feature.MakeRandomK8sName("subscription 1")
	subscription2 := feature.MakeRandomK8sName("subscription 2")
	prober := eventshub.NewProber()
	prober.SetTargetResource(channel_impl.GVR(), channel1)

	f.Setup("install events", eventlibrary.Install(lib))
	f.Setup("use events cache", prober.SenderEventsFromSVC(lib, "events/three.ce"))
	f.Setup("register event expectation", func(ctx context.Context, t feature.T) {
		if err := prober.ExpectYAMLEvents(eventlibrary.PathFor("events/three.ce")); err != nil {
			t.Fatalf("can not find event files: %v", err)
		}
	})

	f.Setup("install sink", prober.ReceiverInstall("sink"))
	f.Setup("install transform service", prober.ReceiverInstall("transform", eventshub.ReplyWithTransformedEvent("transformed", "transformer", "")))
	f.Setup("install channel 1", channel_impl.Install(channel1))
	f.Setup("install channel 2", channel_impl.Install(channel2))
	f.Setup("install subscription 1", subscription.Install(subscription1,
		subscription.WithChannel(channel_impl.AsRef(channel1)),
		subscription.WithSubscriber(prober.AsKReference("transform"), ""),
		subscription.WithReply(channel_impl.AsRef(channel2), ""),
	))
	f.Setup("install subscription 2", subscription.Install(subscription2,
		subscription.WithChannel(channel_impl.AsRef(channel2)),
		subscription.WithSubscriber(prober.AsKReference("sink"), ""),
	))
	f.Setup("subscription 1 is ready", subscription.IsReady(subscription1))
	f.Setup("subscription 2 is ready", subscription.IsReady(subscription2))
	f.Setup("channel 1 is ready", channel_impl.IsReady(channel1))
	f.Setup("channel 2 is ready", channel_impl.IsReady(channel2))
	f.Setup("install source", prober.SenderInstall("source"))
	f.Setup("event library is ready", eventlibrary.IsReady(lib))

	f.Requirement("sender is finished", prober.SenderDone("source"))
	f.Requirement("receiver is finished", prober.ReceiverDone("source", "sink"))

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
		subscription.WithSubscriber(prober.AsKReference("sink"), ""),
	))

	f.Setup("subscription is ready", subscription.IsReady(sub))
	f.Setup("channel is ready", channel_impl.IsReady(channel))
	f.Setup("install source", prober.SenderInstall("source", eventshub.InputEventWithEncoding(event, encoding)))

	f.Requirement("sender is finished", prober.SenderDone("source"))
	f.Requirement("receiver is finished", prober.ReceiverDone("source", "sink"))

	f.Assert("sink receives events", prober.AssertReceivedAll("source", "sink"))

	return f
}
