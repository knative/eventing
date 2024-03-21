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
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/network"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"

	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/pkg/eventingtls/eventingtlstesting"
	"knative.dev/eventing/test/rekt/resources/addressable"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/eventtype"
	"knative.dev/eventing/test/rekt/resources/trigger"

	"knative.dev/reconciler-test/pkg/eventshub/assert"
	eventassert "knative.dev/reconciler-test/pkg/eventshub/assert"

	"knative.dev/eventing/test/rekt/features"
	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/features/source"
	"knative.dev/eventing/test/rekt/resources/pingsource"
)

func SendsEventsWithSinkRef() *feature.Feature {
	source := feature.MakeRandomK8sName("pingsource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	f.Requirement("install pingsource", pingsource.Install(source, pingsource.WithSink(service.AsDestinationRef(sink))))
	f.Requirement("pingsource goes ready", pingsource.IsReady(source))

	f.Stable("pingsource as event source").
		Must("delivers events",
			func(ctx context.Context, t feature.T) {
				assert.OnStore(sink).
					Match(features.HasKnNamespaceHeader(environment.FromContext(ctx).Namespace())).
					MatchEvent(test.HasType("dev.knative.sources.ping")).
					AtLeast(1)(ctx, t)
			},
		)

	return f
}

func SendsEventsTLS() *feature.Feature {
	src := feature.MakeRandomK8sName("pingsource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiverTLS))

	f.Requirement("install pingsource", func(ctx context.Context, t feature.T) {
		d := service.AsDestinationRef(sink)
		d.CACerts = eventshub.GetCaCerts(ctx)

		pingsource.Install(src, pingsource.WithSink(d))(ctx, t)
	})
	f.Requirement("pingsource goes ready", pingsource.IsReady(src))

	f.Stable("pingsource as event source").
		Must("delivers events", assert.OnStore(sink).
			Match(eventassert.MatchKind(eventshub.EventReceived)).
			MatchEvent(test.HasType("dev.knative.sources.ping")).
			AtLeast(1)).
		Must("Set sinkURI to HTTPS endpoint", source.ExpectHTTPSSink(pingsource.Gvr(), src)).
		Must("Set sinkCACerts to non empty CA certs", source.ExpectCACerts(pingsource.Gvr(), src))

	return f
}

func SendsEventsTLSTrustBundle() *feature.Feature {
	src := feature.MakeRandomK8sName("pingsource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install sink", eventshub.Install(sink,
		eventshub.IssuerRef(eventingtlstesting.IssuerKind, eventingtlstesting.IssuerName),
		eventshub.StartReceiverTLS,
	))

	f.Requirement("install pingsource", func(ctx context.Context, t feature.T) {
		d := &duckv1.Destination{
			URI: &apis.URL{
				Scheme: "https", // Force using https
				Host:   network.GetServiceHostname(sink, environment.FromContext(ctx).Namespace()),
			},
			CACerts: nil, // CA certs are in the trust-bundle
		}

		pingsource.Install(src, pingsource.WithSink(d))(ctx, t)
	})
	f.Requirement("pingsource goes ready", pingsource.IsReady(src))

	f.Stable("pingsource as event source").
		Must("delivers events", assert.OnStore(sink).
			Match(eventassert.MatchKind(eventshub.EventReceived)).
			MatchEvent(test.HasType("dev.knative.sources.ping")).
			AtLeast(1)).
		Must("Set sinkURI to HTTPS endpoint", source.ExpectHTTPSSink(pingsource.Gvr(), src))

	return f
}

func SendsEventsWithSinkURI() *feature.Feature {
	source := feature.MakeRandomK8sName("pingsource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	f.Requirement("install pingsource", pingsource.Install(source, pingsource.WithSink(service.AsDestinationRef(sink))))
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

	f.Requirement("install pingsource", pingsource.Install(source,
		pingsource.WithDataBase64("text/plain", "aGVsbG8sIHdvcmxkIQ=="),
		pingsource.WithSink(service.AsDestinationRef(sink)),
	))
	f.Requirement("pingsource goes ready", pingsource.IsReady(source))

	f.Stable("pingsource as event source").
		Must("delivers events", assert.OnStore(sink).MatchEvent(
			test.HasType("dev.knative.sources.ping"),
		).AtLeast(1))

	return f
}

func SendsEventsWithSecondsInSchedule() *feature.Feature {
	source := feature.MakeRandomK8sName("pingsource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	f.Requirement("install pingsource", pingsource.Install(source,
		pingsource.WithSchedule("10 0/1 * * * ?"),
		pingsource.WithSink(service.AsDestinationRef(sink)),
	))
	f.Requirement("pingsource goes ready", pingsource.IsReady(source))

	f.Stable("pingsource as event source").
		Must("delivers events",
			assert.OnStore(sink).MatchEvent(test.HasType("dev.knative.sources.ping")).AtLeast(1))

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
			test.HasType("dev.knative.sources.ping")).AtLeast(1)).
		Must("PingSource test eventtypes match", eventtype.WaitForEventType(
			eventtype.AssertReady(expectedCeTypes),
			eventtype.AssertPresent(expectedCeTypes)))

	return f
}

func SendsEventsWithBrokerAsSinkTLS() *feature.Feature {
	src := feature.MakeRandomK8sName("pingsource")
	brokerName := feature.MakeRandomK8sName("broker")
	sinkName := feature.MakeRandomK8sName("sink")
	triggerName := feature.MakeRandomK8sName("trigger")
	f := feature.NewFeature()

	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("broker is ready", broker.IsReady(brokerName))
	f.Setup("broker is addressable", broker.IsAddressable(brokerName))
	f.Setup("Broker has HTTPS address", broker.ValidateAddress(brokerName, addressable.AssertHTTPSAddress))

	f.Setup("install sink", eventshub.Install(sinkName, eventshub.StartReceiverTLS))

	f.Setup("install trigger", func(ctx context.Context, t feature.T) {
		d := service.AsDestinationRef(sinkName)
		d.CACerts = eventshub.GetCaCerts(ctx)
		trigger.Install(triggerName, brokerName, trigger.WithSubscriberFromDestination(d))(ctx, t)
	})
	f.Setup("Wait for Trigger to become ready", trigger.IsReady(triggerName))

	f.Requirement("install PingSource", pingsource.Install(src, pingsource.WithData("text/plain", "hello, world!"), pingsource.WithSink(broker.AsDestinationRef(brokerName))))
	f.Requirement("PingSource goes ready", pingsource.IsReady(src))

	f.Assert("PingSource has HTTPS sink address", source.ExpectHTTPSSink(pingsource.Gvr(), src))
	f.Assert("PingSource as event source",
		assert.OnStore(sinkName).MatchEvent(test.HasType("dev.knative.sources.ping")).AtLeast(1))

	return f

}
