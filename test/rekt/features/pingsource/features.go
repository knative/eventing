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
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/resources/service"

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
