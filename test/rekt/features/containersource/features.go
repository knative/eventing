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

package containersource

import (
	"context"
	"fmt"

	"github.com/cloudevents/sdk-go/v2/test"
	"k8s.io/apimachinery/pkg/util/uuid"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/manifest"

	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/resources/service"

	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/resources/containersource"
)

func SendsEventsWithSinkRef() *feature.Feature {
	source := feature.MakeRandomK8sName("containersource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	f.Requirement("install containersource", containersource.Install(source, containersource.WithSink(service.AsDestinationRef(sink))))
	f.Requirement("containersource goes ready", containersource.IsReady(source))

	f.Stable("containersource as event source").
		Must("delivers events",
			assert.OnStore(sink).MatchEvent(test.HasType("dev.knative.eventing.samples.heartbeat")).AtLeast(1))

	return f
}

func SendsEventsWithSinkRefOIDC() *feature.Feature {
	source := feature.MakeRandomK8sName("containersource")
	sink := feature.MakeRandomK8sName("sink")
	sinkAudience := "audience"
	f := feature.NewFeature()

	f.Setup("install sink", eventshub.Install(sink,
		eventshub.OIDCReceiverAudience(sinkAudience),
		eventshub.StartReceiver))

	f.Requirement("install containersource", containersource.Install(source,
		containersource.WithSink(&duckv1.Destination{
			Ref:      service.AsKReference(sink),
			Audience: &sinkAudience,
		})))
	f.Requirement("containersource goes ready", containersource.IsReady(source))

	f.Stable("containersource as event source").
		Must("delivers events",
			assert.OnStore(sink).MatchEvent(test.HasType("dev.knative.eventing.samples.heartbeat")).AtLeast(1))

	return f
}

func SendsEventsWithSinkURI() *feature.Feature {
	source := feature.MakeRandomK8sName("containersource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	f.Requirement("install containersource", containersource.Install(source, containersource.WithSink(service.AsDestinationRef(sink))))
	f.Requirement("containersource goes ready", containersource.IsReady(source))

	f.Stable("containersource as event source").
		Must("delivers events",
			assert.OnStore(sink).MatchEvent(test.HasType("dev.knative.eventing.samples.heartbeat")).AtLeast(1))

	return f
}

func SendsEventsWithCloudEventOverrides() *feature.Feature {
	source := feature.MakeRandomK8sName("containersource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()
	extensions := map[string]interface{}{
		"wow": "so extended",
	}

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	f.Requirement("install containersource", containersource.Install(source,
		containersource.WithSink(service.AsDestinationRef(sink)),
		containersource.WithExtensions(extensions),
		manifest.WithPodAnnotations(map[string]interface{}{
			"foo": true,
		}),
	))
	f.Requirement("containersource goes ready", containersource.IsReady(source))

	f.Stable("containersource as event source").
		Must("delivers events", assert.OnStore(sink).MatchEvent(
			test.HasType("dev.knative.eventing.samples.heartbeat"),
			test.HasExtensions(extensions),
		).AtLeast(1))

	return f
}

func SendsEventsWithArgs() *feature.Feature {
	source := feature.MakeRandomK8sName("containersource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	message := fmt.Sprintf("msg %s for TestContainerSource", uuid.NewUUID())
	args := "--msg=" + message

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	f.Requirement("install containersource", containersource.Install(source,
		containersource.WithSink(service.AsDestinationRef(sink)),
		containersource.WithArgs(args),
	))
	f.Requirement("containersource goes ready", containersource.IsReady(source))

	f.Stable("containersource as event source to test args message").
		Must("delivers events", assert.OnStore(sink).MatchEvent(
			test.HasType("dev.knative.eventing.samples.heartbeat"),
			assert.MatchHeartBeatsImageMessage(message),
		).AtLeast(1))

	return f
}

func SendEventsWithTLSRecieverAsSink() *feature.Feature {
	source := feature.MakeRandomK8sName("containersource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiverTLS))

	f.Requirement("install ContainerSource", func(ctx context.Context, t feature.T) {
		d := service.AsDestinationRef(sink)
		d.CACerts = eventshub.GetCaCerts(ctx)

		containersource.Install(source, containersource.WithSink(d))(ctx, t)
	})
	f.Requirement("containersource goes ready", containersource.IsReady(source))

	f.Stable("containersource as event source").
		Must("delivers events",
			assert.OnStore(sink).
				Match(assert.MatchKind(eventshub.EventReceived)).
				MatchEvent(test.HasType("dev.knative.eventing.samples.heartbeat")).
				AtLeast(1),
		)

	return f
}
