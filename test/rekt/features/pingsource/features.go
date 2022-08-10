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

package pingsource

import (
	"context"

	"github.com/cloudevents/sdk-go/v2/test"
	"k8s.io/apimachinery/pkg/util/sets"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/eventtype"
	"knative.dev/eventing/test/rekt/resources/pingsource"
	"knative.dev/eventing/test/rekt/resources/trigger"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/resources/svc"
)

func SendsEventsWithSinkRef() *feature.Feature {
	source := feature.MakeRandomK8sName("pingsource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	f.Setup("install pingsource", pingsource.Install(source, pingsource.WithSink(svc.AsKReference(sink), "")))
	f.Requirement("pingsource goes ready", pingsource.IsReady(source))

	f.Stable("pingsource as event source").
		Must("delivers events",
			assert.OnStore(sink).MatchEvent(test.HasType("dev.knative.sources.ping")).AtLeast(1))

	return f
}

func SendsEventsWithSinkURI() *feature.Feature {
	source := feature.MakeRandomK8sName("pingsource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	f.Setup("install pingsource", func(ctx context.Context, t feature.T) {
		uri, err := svc.Address(ctx, sink)
		if err != nil {
			t.Error("failed to get address of sink", err)
		}
		pingsource.Install(source, pingsource.WithSink(nil, uri.String()))(ctx, t)
	})
	f.Requirement("pingsource goes ready", pingsource.IsReady(source))

	f.Stable("pingsource as event source").
		Must("delivers events",
			assert.OnStore(sink).MatchEvent(test.HasType("dev.knative.sources.ping")).AtLeast(1))

	return f
}

func SendsEventsWithCloudEventData() *feature.Feature {
	source := feature.MakeRandomK8sName("pingsource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	cfg := []manifest.CfgFn{
		pingsource.WithDataBase64("text/plain", "aGVsbG8sIHdvcmxkIQ=="),
		pingsource.WithSink(&duckv1.KReference{
			Kind:       "Service",
			Name:       sink,
			APIVersion: "v1",
		}, ""),
	}
	f.Setup("install pingsource", pingsource.Install(source, cfg...))

	f.Requirement("pingsource goes ready", pingsource.IsReady(source))

	f.Stable("pingsource as event source").
		Must("delivers events", assert.OnStore(sink).MatchEvent(
			test.HasType("dev.knative.sources.ping"),
		).AtLeast(1))

	return f
}

// SendsEventsWithEventTypes tests pingsource to a ready broker.
func SendsEventsWithEventTypes() *feature.Feature {
	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	via := feature.MakeRandomK8sName("via")

	f := new(feature.Feature)

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

	f.Setup("install pingsource", func(ctx context.Context, t feature.T) {
		brokeruri, err := broker.Address(ctx, brokerName)
		if err != nil {
			t.Error("failed to get address of broker", err)
		}
		cfg := []manifest.CfgFn{
			pingsource.WithSink(nil, brokeruri.String()),
			pingsource.WithData("text/plain", "hello, world!"),
		}
		pingsource.Install(source, cfg...)(ctx, t)
	})
	f.Setup("PingSource goes ready", pingsource.IsReady(source))

	expectedCeTypes := sets.NewString(sourcesv1.PingSourceEventType)

	f.Stable("pingsource as event source").
		Must("delivers events on broker with URI", assert.OnStore(sink).MatchEvent(
			test.HasType("dev.knative.sources.ping")).AtLeast(1)).
		Must("PingSource test eventtypes match", eventtype.WaitForEventType(
			eventtype.AssertPresent(expectedCeTypes)))

	return f
}
