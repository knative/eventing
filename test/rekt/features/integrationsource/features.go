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
	source := feature.MakeRandomK8sName("integrationsource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	f.Requirement("install integrationsource", integrationsource.Install(source, integrationsource.WithSink(service.AsDestinationRef(sink))))
	f.Requirement("integrationsource goes ready", integrationsource.IsReady(source))

	f.Stable("integrationsource as event source").
		Must("delivers events",
			assert.OnStore(sink).MatchEvent(test.HasType("dev.knative.connector.event.timer")).AtLeast(1))

	return f
}

func SendEventsWithTLSRecieverAsSink() *feature.Feature {
	src := feature.MakeRandomK8sName("integrationsource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiverTLS))

	f.Requirement("install ContainerSource", func(ctx context.Context, t feature.T) {
		d := service.AsDestinationRef(sink)
		d.CACerts = eventshub.GetCaCerts(ctx)

		integrationsource.Install(src, integrationsource.WithSink(d))(ctx, t)
	})
	f.Requirement("integrationsource goes ready", integrationsource.IsReady(src))

	f.Stable("integrationsource as event source").
		Must("delivers events",
			assert.OnStore(sink).
				Match(assert.MatchKind(eventshub.EventReceived)).
				MatchEvent(test.HasType("dev.knative.connector.event.timer")).
				AtLeast(1),
		).
		Must("Set sinkURI to HTTPS endpoint", source.ExpectHTTPSSink(integrationsource.Gvr(), src)).
		Must("Set sinkCACerts to non empty CA certs", source.ExpectCACerts(integrationsource.Gvr(), src))

	return f
}

func SendEventsWithTLSRecieverAsSinkTrustBundle() *feature.Feature {
	src := feature.MakeRandomK8sName("integrationsource")
	sink := feature.MakeRandomK8sName("sink")
	f := feature.NewFeature()

	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install sink", eventshub.Install(sink,
		eventshub.IssuerRef(eventingtlstesting.IssuerKind, eventingtlstesting.IssuerName),
		eventshub.StartReceiverTLS,
	))

	f.Requirement("install ContainerSource", func(ctx context.Context, t feature.T) {
		integrationsource.Install(src, integrationsource.WithSink(&duckv1.Destination{
			URI: &apis.URL{
				Scheme: "https", // Force using https
				Host:   network.GetServiceHostname(sink, environment.FromContext(ctx).Namespace()),
			},
			CACerts: nil, // CA certs are in the trust-bundle
		}))(ctx, t)
	})
	f.Requirement("integrationsource goes ready", integrationsource.IsReady(src))

	f.Stable("integrationsource as event source").
		Must("delivers events",
			assert.OnStore(sink).
				Match(assert.MatchKind(eventshub.EventReceived)).
				MatchEvent(test.HasType("dev.knative.connector.event.timer")).
				AtLeast(1),
		).
		Must("Set sinkURI to HTTPS endpoint", source.ExpectHTTPSSink(integrationsource.Gvr(), src))

	return f
}
