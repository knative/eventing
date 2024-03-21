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

package channel

import (
	"context"
	"time"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/network"
	"knative.dev/pkg/system"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/resources/service"
	"knative.dev/reconciler-test/resources/certificate"

	"knative.dev/eventing/pkg/eventingtls/eventingtlstesting"
	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/resources/addressable"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/eventing/test/rekt/resources/subscription"
)

func RotateDispatcherTLSCertificate() *feature.Feature {
	certificateName := "imc-dispatcher-server-tls"
	secretName := "imc-dispatcher-server-tls"

	channelName := feature.MakeRandomK8sName("channel")
	subscriptionName := feature.MakeRandomK8sName("sub")
	sink := feature.MakeRandomK8sName("sink")
	source := feature.MakeRandomK8sName("source")

	f := feature.NewFeatureNamed("Rotate " + certificateName + " certificate")

	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("Rotate certificate", certificate.Rotate(certificate.RotateCertificate{
		Certificate: types.NamespacedName{
			Namespace: system.Namespace(),
			Name:      certificateName,
		},
	}))

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiverTLS))
	f.Setup("install channel", channel_impl.Install(channelName))
	f.Setup("channel is ready", channel_impl.IsReady(channelName))
	f.Setup("install subscription", func(ctx context.Context, t feature.T) {
		d := service.AsDestinationRef(sink)
		d.CACerts = eventshub.GetCaCerts(ctx)
		subscription.Install(subscriptionName,
			subscription.WithChannel(channel_impl.AsRef(channelName)),
			subscription.WithSubscriberFromDestination(d))(ctx, t)
	})
	f.Setup("subscription is ready", subscription.IsReady(subscriptionName))
	f.Setup("Channel has HTTPS address", channel_impl.ValidateAddress(channelName, addressable.AssertHTTPSAddress))

	event := cetest.FullEvent()
	event.SetID(uuid.New().String())

	f.Requirement("install source", eventshub.Install(source,
		eventshub.StartSenderToResourceTLS(channel_impl.GVR(), channelName, nil),
		eventshub.InputEvent(event),
		// Send multiple events so that we take into account that the certificate rotation might
		// be detected by the server after some time.
		eventshub.SendMultipleEvents(100, 3*time.Second),
	))

	f.Assert("Event sent", assert.OnStore(source).
		MatchSentEvent(cetest.HasId(event.ID())).
		AtLeast(1),
	)
	f.Assert("Event received", assert.OnStore(sink).
		MatchReceivedEvent(cetest.HasId(event.ID())).
		AtLeast(1),
	)
	f.Assert("Source match updated peer certificate", assert.OnStore(source).
		MatchPeerCertificatesReceived(assert.MatchPeerCertificatesFromSecret(system.Namespace(), secretName, "tls.crt")).
		AtLeast(1),
	)

	return f
}

func SubscriptionTLS() *feature.Feature {

	channelName := feature.MakeRandomK8sName("channel")
	subscriptionName := feature.MakeRandomK8sName("sub")
	sink := feature.MakeRandomK8sName("sink")
	source := feature.MakeRandomK8sName("source")
	dlsName := feature.MakeRandomK8sName("dls")
	dlsSubscriptionName := feature.MakeRandomK8sName("dls-sub")

	f := feature.NewFeature()

	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiverTLS))
	f.Setup("install dead letter sink", eventshub.Install(dlsName, eventshub.StartReceiverTLS))
	f.Setup("install channel", channel_impl.Install(channelName))
	f.Setup("channel is ready", channel_impl.IsReady(channelName))
	f.Setup("install subscription", func(ctx context.Context, t feature.T) {
		d := service.AsDestinationRef(sink)
		d.CACerts = eventshub.GetCaCerts(ctx)
		subscription.Install(subscriptionName,
			subscription.WithChannel(channel_impl.AsRef(channelName)),
			subscription.WithSubscriberFromDestination(d))(ctx, t)
	})
	f.Setup("subscription is ready", subscription.IsReady(subscriptionName))
	f.Setup("install dead letter subscription", func(ctx context.Context, t feature.T) {
		d := service.AsDestinationRef(dlsName)
		d.CACerts = eventshub.GetCaCerts(ctx)
		subscription.Install(dlsSubscriptionName,
			subscription.WithChannel(channel_impl.AsRef(channelName)),
			subscription.WithDeadLetterSinkFromDestination(d),
			subscription.WithSubscriber(nil, "http://127.0.0.1:2468", ""))(ctx, t)
	})
	f.Setup("subscription dead letter is ready", subscription.IsReady(dlsSubscriptionName))
	f.Setup("Channel has HTTPS address", channel_impl.ValidateAddress(channelName, addressable.AssertHTTPSAddress))

	event := cetest.FullEvent()
	event.SetID(uuid.New().String())

	f.Requirement("install source", eventshub.Install(source,
		eventshub.StartSenderToResourceTLS(channel_impl.GVR(), channelName, nil),
		eventshub.InputEvent(event),
	))

	f.Assert("Event sent", assert.OnStore(source).
		MatchSentEvent(cetest.HasId(event.ID())).
		AtLeast(1),
	)
	f.Assert("Event received in sink", assert.OnStore(sink).
		MatchReceivedEvent(cetest.HasId(event.ID())).
		AtLeast(1),
	)
	f.Assert("Event received in dead letter sink", assert.OnStore(dlsName).
		MatchReceivedEvent(cetest.HasId(event.ID())).
		AtLeast(1),
	)

	return f
}

func SubscriptionTLSTrustBundle() *feature.Feature {

	channelName := feature.MakeRandomK8sName("channel")
	subscriptionName := feature.MakeRandomK8sName("sub")
	sink := feature.MakeRandomK8sName("sink")
	source := feature.MakeRandomK8sName("source")
	dlsName := feature.MakeRandomK8sName("dls")
	dlsSubscriptionName := feature.MakeRandomK8sName("dls-sub")

	f := feature.NewFeature()

	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install sink", eventshub.Install(sink,
		eventshub.IssuerRef(eventingtlstesting.IssuerKind, eventingtlstesting.IssuerName),
		eventshub.StartReceiverTLS,
	))
	f.Setup("install sink", eventshub.Install(dlsName,
		eventshub.IssuerRef(eventingtlstesting.IssuerKind, eventingtlstesting.IssuerName),
		eventshub.StartReceiverTLS,
	))
	f.Setup("install channel", channel_impl.Install(channelName))
	f.Setup("channel is ready", channel_impl.IsReady(channelName))
	f.Setup("install subscription", func(ctx context.Context, t feature.T) {
		d := &duckv1.Destination{
			URI: &apis.URL{
				Scheme: "https", // Force using https
				Host:   network.GetServiceHostname(sink, environment.FromContext(ctx).Namespace()),
			},
			CACerts: nil, // CA certs are in the trust-bundle
		}
		subscription.Install(subscriptionName,
			subscription.WithChannel(channel_impl.AsRef(channelName)),
			subscription.WithSubscriberFromDestination(d))(ctx, t)
	})
	f.Setup("subscription is ready", subscription.IsReady(subscriptionName))
	f.Setup("install dead letter subscription", func(ctx context.Context, t feature.T) {
		d := &duckv1.Destination{
			URI: &apis.URL{
				Scheme: "https", // Force using https
				Host:   network.GetServiceHostname(dlsName, environment.FromContext(ctx).Namespace()),
			},
			CACerts: nil, // CA certs are in the trust-bundle
		}

		subscription.Install(dlsSubscriptionName,
			subscription.WithChannel(channel_impl.AsRef(channelName)),
			subscription.WithDeadLetterSinkFromDestination(d),
			subscription.WithSubscriber(nil, "http://127.0.0.1:2468", ""))(ctx, t)
	})
	f.Setup("subscription dead letter is ready", subscription.IsReady(dlsSubscriptionName))
	f.Setup("Channel has HTTPS address", channel_impl.ValidateAddress(channelName, addressable.AssertHTTPSAddress))

	event := cetest.FullEvent()
	event.SetID(uuid.New().String())

	f.Requirement("install source", eventshub.Install(source,
		eventshub.StartSenderToResourceTLS(channel_impl.GVR(), channelName, nil),
		eventshub.InputEvent(event),
		// Send multiple events so that we take into account that the certificate rotation might
		// be detected by the server after some time.
		eventshub.SendMultipleEvents(100, 3*time.Second),
	))

	f.Assert("Event sent", assert.OnStore(source).
		MatchSentEvent(cetest.HasId(event.ID())).
		AtLeast(1),
	)
	f.Assert("Event received in sink", assert.OnStore(sink).
		MatchReceivedEvent(cetest.HasId(event.ID())).
		AtLeast(1),
	)
	f.Assert("Event received in dead letter sink", assert.OnStore(dlsName).
		MatchReceivedEvent(cetest.HasId(event.ID())).
		AtLeast(1),
	)

	return f
}
