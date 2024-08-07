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

package broker

import (
	"context"

	"github.com/cloudevents/sdk-go/v2/test"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/test/rekt/resources/eventpolicy"
	"knative.dev/eventing/test/rekt/resources/pingsource"
	"knative.dev/reconciler-test/pkg/environment"

	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/trigger"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/resources/service"
)

func BrokerSupportsAuthZ() *feature.FeatureSet {
	return &feature.FeatureSet{
		Name: "Broker supports authorization",
		Features: []*feature.Feature{
			BrokerAcceptsEventsFromAuthorizedSender(),
			BrokerRejectsEventsFromUnauthorizedSender(),
		},
	}
}

func BrokerAcceptsEventsFromAuthorizedSender() *feature.Feature {
	f := feature.NewFeatureNamed("Broker accepts events from a authorized sender")

	f.Prerequisite("OIDC Authentication is enabled", featureflags.AuthenticationOIDCEnabled())
	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	source := feature.MakeRandomK8sName("source")
	brokerName := feature.MakeRandomK8sName("broker")
	sink := feature.MakeRandomK8sName("sink")
	triggerName := feature.MakeRandomK8sName("trigger")
	eventPolicyName := feature.MakeRandomK8sName("eventpolicy")

	// Install the broker
	f.Setup("Install Broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("Broker is ready", broker.IsReady(brokerName))
	f.Setup("Broker is addressable", broker.IsAddressable(brokerName))

	// Install the sink
	f.Setup("Install Sink", eventshub.Install(
		sink,
		eventshub.StartReceiver,
	))

	f.Setup("Install the Trigger", trigger.Install(triggerName,
		trigger.WithBrokerName(brokerName),
		trigger.WithSubscriber(service.AsKReference(sink), "")))

	f.Setup("Install the EventPolicy", func(ctx context.Context, t feature.T) {
		eventpolicy.Install(
			eventPolicyName,
			eventpolicy.WithToRef(
				broker.GVR().GroupVersion().WithKind("Broker"),
				brokerName),
			eventpolicy.WithFromRef(
				pingsource.Gvr().GroupVersion().WithKind("PingSource"),
				source,
				environment.FromContext(ctx).Namespace()),
		)(ctx, t)
	})

	// Install source
	f.Requirement("Install Pingsource", func(ctx context.Context, t feature.T) {
		brokeruri, err := broker.Address(ctx, brokerName)
		if err != nil {
			t.Error("failed to get address of broker", err)
		}

		pingsource.Install(source,
			pingsource.WithSink(&duckv1.Destination{URI: brokeruri.URL, CACerts: brokeruri.CACerts, Audience: brokeruri.Audience}),
			pingsource.WithData("text/plain", "hello, world!"))(ctx, t)
	})
	f.Requirement("PingSource goes ready", pingsource.IsReady(source))

	f.Alpha("Broker").
		Must("accepts event from valid sender", assert.OnStore(sink).MatchEvent(
			test.HasType(sourcesv1.PingSourceEventType)).AtLeast(1))

	return f
}

func BrokerRejectsEventsFromUnauthorizedSender() *feature.Feature {
	f := feature.NewFeatureNamed("Broker rejects events from an unauthorized sender")

	f.Prerequisite("OIDC Authentication is enabled", featureflags.AuthenticationOIDCEnabled())
	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	source := feature.MakeRandomK8sName("source")
	brokerName := feature.MakeRandomK8sName("broker")
	sink := feature.MakeRandomK8sName("sink")
	triggerName := feature.MakeRandomK8sName("trigger")
	eventPolicyName := feature.MakeRandomK8sName("eventpolicy")

	event := test.FullEvent()

	// Install the broker
	f.Setup("Install Broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("Broker is ready", broker.IsReady(brokerName))
	f.Setup("Broker is addressable", broker.IsAddressable(brokerName))

	// Install the sink
	f.Setup("Install Sink", eventshub.Install(
		sink,
		eventshub.StartReceiver,
	))

	f.Setup("Install the Trigger", trigger.Install(triggerName,
		trigger.WithBrokerName(brokerName),
		trigger.WithSubscriber(service.AsKReference(sink), "")))
	f.Setup("Trigger goes ready", trigger.IsReady(triggerName))

	// Install an event policy for Broker allowing from a sample subject, to not fall back to the default-auth-mode
	f.Setup("Install an EventPolicy", eventpolicy.Install(
		eventPolicyName,
		eventpolicy.WithToRef(
			broker.GVR().GroupVersion().WithKind("Broker"),
			brokerName),
		eventpolicy.WithFromSubject("sample-sub")))

	// Send event
	f.Requirement("Install Source", eventshub.Install(
		source,
		eventshub.StartSenderToResourceTLS(broker.GVR(), brokerName, nil),
		eventshub.InputEvent(event),
	))

	f.Alpha("Broker").
		Must("event is sent", assert.OnStore(source).MatchSentEvent(
			test.HasId(event.ID())).Exact(1)).
		Must("broker rejects event with a 403 response", assert.OnStore(source).Match(assert.MatchStatusCode(403)).Exact(1))

	return f
}
