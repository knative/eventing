/*
Copyright 2024 The Knative Authors

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

package integrationsource

import (
	"context"

	"github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/eventing/pkg/eventingtls/eventingtlstesting"
	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/features/source"
	"knative.dev/eventing/test/rekt/resources/integrationsource"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/network"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/resources/service"
)

func SendsEventsWithSinkRef() *feature.Feature {
	sourceName := feature.MakeRandomK8sName("integrationsource")
	sinkName := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Setup("install sink", eventshub.Install(sinkName, eventshub.StartReceiver))

	f.Requirement("install integrationsource", integrationsource.Install(sourceName, integrationsource.WithSink(service.AsDestinationRef(sinkName))))
	f.Requirement("integrationsource goes ready", integrationsource.IsReady(sourceName))

	f.Stable("integrationsource as event source").
		Must("delivers events",
			assert.OnStore(sinkName).MatchEvent(test.HasType("dev.knative.eventing.timer")).AtLeast(1))

	return f
}

func SendEventsWithTLSRecieverAsSink() *feature.Feature {
	sourceName := feature.MakeRandomK8sName("integrationsource")
	sinkName := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install sink", eventshub.Install(sinkName, eventshub.StartReceiverTLS))

	f.Requirement("install ContainerSource", func(ctx context.Context, t feature.T) {
		d := service.AsDestinationRef(sinkName)
		d.CACerts = eventshub.GetCaCerts(ctx)

		integrationsource.Install(sourceName, integrationsource.WithSink(d))(ctx, t)
	})
	f.Requirement("integrationsource goes ready", integrationsource.IsReady(sourceName))

	f.Stable("integrationsource as event source").
		Must("delivers events",
			assert.OnStore(sinkName).
				Match(assert.MatchKind(eventshub.EventReceived)).
				MatchEvent(test.HasType("dev.knative.eventing.timer")).
				AtLeast(1),
		).
		Must("Set sinkURI to HTTPS endpoint", source.ExpectHTTPSSink(integrationsource.Gvr(), sourceName)).
		Must("Set sinkCACerts to non empty CA certs", source.ExpectCACerts(integrationsource.Gvr(), sourceName))

	return f
}

func SendEventsWithTLSRecieverAsSinkTrustBundle() *feature.Feature {
	sourceName := feature.MakeRandomK8sName("integrationsource")
	sinkName := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install sink", eventshub.Install(sinkName,
		eventshub.IssuerRef(eventingtlstesting.IssuerKind, eventingtlstesting.IssuerName),
		eventshub.StartReceiverTLS,
	))

	f.Requirement("install ContainerSource", func(ctx context.Context, t feature.T) {
		integrationsource.Install(sourceName, integrationsource.WithSink(&duckv1.Destination{
			URI: &apis.URL{
				Scheme: "https", // Force using https
				Host:   network.GetServiceHostname(sinkName, environment.FromContext(ctx).Namespace()),
			},
			CACerts: nil, // CA certs are in the trust-bundle
		}))(ctx, t)
	})
	f.Requirement("integrationsource goes ready", integrationsource.IsReady(sourceName))

	f.Stable("integrationsource as event source").
		Must("delivers events",
			assert.OnStore(sinkName).
				Match(assert.MatchKind(eventshub.EventReceived)).
				MatchEvent(test.HasType("dev.knative.eventing.timer")).
				AtLeast(1),
		).
		Must("Set sinkURI to HTTPS endpoint", source.ExpectHTTPSSink(integrationsource.Gvr(), sourceName))

	return f
}
