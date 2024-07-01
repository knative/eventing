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

package broker

import (
	"context"
	"time"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/system"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/resources/service"
	"knative.dev/reconciler-test/resources/certificate"

	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/resources/addressable"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/trigger"
)

func RotateMTChannelBrokerTLSCertificates() *feature.Feature {
	ingressCertificateName := "mt-broker-ingress-server-tls"
	ingressSecretName := "mt-broker-ingress-server-tls"

	filterCertificateName := "mt-broker-filter-server-tls"

	brokerName := feature.MakeRandomK8sName("broker")
	triggerName := feature.MakeRandomK8sName("trigger")
	sink := feature.MakeRandomK8sName("sink")
	source := feature.MakeRandomK8sName("source")

	f := feature.NewFeatureNamed("Rotate MTChannelBroker TLS certificate")

	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("Rotate ingress certificate", certificate.Rotate(certificate.RotateCertificate{
		Certificate: types.NamespacedName{
			Namespace: system.Namespace(),
			Name:      ingressCertificateName,
		},
	}))
	// We cannot externally verify this certificate rotation
	f.Setup("Rotate filter certificate", certificate.Rotate(certificate.RotateCertificate{
		Certificate: types.NamespacedName{
			Namespace: system.Namespace(),
			Name:      filterCertificateName,
		},
	}))

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiverTLS))
	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("Broker is ready", broker.IsReady(brokerName))
	f.Setup("install trigger", func(ctx context.Context, t feature.T) {
		d := service.AsDestinationRef(sink)
		d.CACerts = eventshub.GetCaCerts(ctx)
		trigger.Install(triggerName, trigger.WithBrokerName(brokerName), trigger.WithSubscriberFromDestination(d))(ctx, t)
	})
	f.Setup("trigger is ready", trigger.IsReady(triggerName))
	f.Setup("Broker has HTTPS address", broker.ValidateAddress(brokerName, addressable.AssertHTTPSAddress))

	event := cetest.FullEvent()
	event.SetID(uuid.New().String())

	f.Requirement("install source", eventshub.Install(source,
		eventshub.StartSenderToResourceTLS(broker.GVR(), brokerName, nil),
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
		MatchPeerCertificatesReceived(assert.MatchPeerCertificatesFromSecret(system.Namespace(), ingressSecretName, "tls.crt")).
		AtLeast(1),
	)

	return f
}
