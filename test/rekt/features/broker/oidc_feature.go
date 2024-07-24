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

	"knative.dev/pkg/apis"

	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/delivery"
	"knative.dev/eventing/test/rekt/resources/trigger"
	triggerresources "knative.dev/eventing/test/rekt/resources/trigger"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/eventshub"
	eventassert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/resources/service"
)

func BrokerSendEventWithOIDC() *feature.FeatureSet {
	return &feature.FeatureSet{
		Name: "Broker send events with OIDC support",
		Features: []*feature.Feature{
			BrokerSendEventWithOIDCTokenToSubscriber(),
			BrokerSendEventWithOIDCTokenToReply(),
			BrokerSendEventWithOIDCTokenToDLS(),
		},
	}
}

func BrokerSendEventWithOIDCTokenToSubscriber() *feature.Feature {
	f := feature.NewFeatureNamed("Broker supports flow with OIDC tokens")

	f.Prerequisite("OIDC Authentication is enabled", featureflags.AuthenticationOIDCEnabled())
	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	source := feature.MakeRandomK8sName("source")
	brokerName := feature.MakeRandomK8sName("broker")
	sink := feature.MakeRandomK8sName("sink")
	triggerName := feature.MakeRandomK8sName("triggerName")
	sinkAudience := "sink-audience"

	event := test.FullEvent()

	// Install the broker
	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("broker is ready", broker.IsReady(brokerName))
	f.Setup("broker is addressable", broker.IsAddressable(brokerName))

	// Install the sink
	f.Setup("install sink", eventshub.Install(
		sink,
		eventshub.StartReceiverTLS,
		eventshub.OIDCReceiverAudience(sinkAudience),
	))

	f.Setup("Install the trigger", func(ctx context.Context, t feature.T) {
		d := service.AsDestinationRef(sink)
		d.CACerts = eventshub.GetCaCerts(ctx)
		d.Audience = &sinkAudience
		trigger.Install(triggerName, trigger.WithBrokerName(brokerName), trigger.WithSubscriberFromDestination(d))(ctx, t)
	})
	f.Setup("trigger goes ready", trigger.IsReady(triggerName))

	// Send event
	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResourceTLS(broker.GVR(), brokerName, nil),
		eventshub.InputEvent(event),
	))

	f.Alpha("Broker").
		Must("handles event with valid OIDC token", eventassert.OnStore(sink).MatchReceivedEvent(test.HasId(event.ID())).Exact(1)).
		Must("uses triggers identity for OIDC", eventassert.OnStore(sink).MatchWithContext(
			eventassert.MatchKind(eventshub.EventReceived).WithContext(),
			eventassert.MatchOIDCUserFromResource(triggerresources.GVR(), triggerName)).AtLeast(1))

	return f
}

func BrokerSendEventWithOIDCTokenToDLS() *feature.Feature {
	f := feature.NewFeature()

	f.Prerequisite("OIDC Authentication is enabled", featureflags.AuthenticationOIDCEnabled())
	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	brokerName := feature.MakeRandomK8sName("broker")
	dls := feature.MakeRandomK8sName("dls")
	triggerName := feature.MakeRandomK8sName("trigger")
	source := feature.MakeRandomK8sName("source")
	dlsAudience := "dls-audience"

	event := test.FullEvent()
	event.SetID(uuid.New().String())

	// Install DLS sink
	f.Setup("install dead letter sink", eventshub.Install(dls,
		eventshub.OIDCReceiverAudience(dlsAudience),
		eventshub.StartReceiverTLS))

	f.Setup("install broker", func(ctx context.Context, t feature.T) {
		brokerConfig := append(broker.WithEnvConfig(),
			delivery.WithDeadLetterSinkFromDestination(&duckv1.Destination{
				Ref:      service.AsKReference(dls),
				Audience: &dlsAudience,
				CACerts:  eventshub.GetCaCerts(ctx),
			}))
		broker.Install(brokerName, brokerConfig...)(ctx, t)
	})

	f.Setup("Broker is ready", broker.IsReady(brokerName))

	f.Setup("Install the trigger", func(ctx context.Context, t feature.T) {
		// create an empty destination ref
		d := duckv1.Destination{}
		d.CACerts = eventshub.GetCaCerts(ctx)
		d.URI, _ = apis.ParseURL("bad://uri")
		trigger.Install(triggerName, trigger.WithBrokerName(brokerName), trigger.WithSubscriberFromDestination(&d))(ctx, t)

	})

	f.Setup("trigger is ready", trigger.IsReady(triggerName))

	// Send events after data plane is ready.
	f.Requirement("install source", eventshub.Install(source,
		eventshub.StartSenderToResourceTLS(broker.GVR(), brokerName, nil),
		eventshub.InputEvent(event),
	))

	// Assert events ended up where we expected.
	f.Stable("broker with DLS").
		Must("deliver event to DLQ", eventassert.OnStore(dls).MatchReceivedEvent(test.HasId(event.ID())).AtLeast(1)).
		Must("uses triggers identity for OIDC", eventassert.OnStore(dls).MatchWithContext(
			eventassert.MatchKind(eventshub.EventReceived).WithContext(),
			eventassert.MatchOIDCUserFromResource(triggerresources.GVR(), triggerName)).AtLeast(1))

	return f
}

func BrokerSendEventWithOIDCTokenToReply() *feature.Feature {
	//1. An event is sent to a broker.
	//2. A trigger routes this event to a subscriber.
	//3. The subscriber processes and replies to the event.
	//4. A helper trigger routes the reply to a designated sink.
	//5. The test verifies that the reply reaches the sink with the expected modifications.
	f := feature.NewFeature()

	f.Prerequisite("OIDC Authentication is enabled", featureflags.AuthenticationOIDCEnabled())
	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	brokerName := feature.MakeRandomK8sName("broker")
	subscriber := feature.MakeRandomK8sName("subscriber")
	reply := feature.MakeRandomK8sName("reply")
	triggerName := feature.MakeRandomK8sName("trigger")
	helperTriggerName := feature.MakeRandomK8sName("helper-trigger")
	source := feature.MakeRandomK8sName("source")

	event := test.FullEvent()
	event.SetID(uuid.New().String())

	replyEventType := "reply-type"
	replyEventSource := "reply-source"

	// Install subscriber
	f.Setup("install subscriber", eventshub.Install(subscriber,
		eventshub.ReplyWithTransformedEvent(replyEventType, replyEventSource, ""),
		eventshub.StartReceiverTLS))

	// Install sink for reply
	// Hint: we don't need to require OIDC auth at the reply sink, because the
	// actual reply is sent to the broker ingress, which must support OIDC. This
	// reply sink is only to check that the reply was sent and routed correctly.
	f.Setup("install sink for reply", eventshub.Install(reply,
		eventshub.StartReceiverTLS))

	// Install broker
	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("Broker is ready", broker.IsReady(brokerName))

	f.Setup("install the trigger", func(ctx context.Context, t feature.T) {
		d := service.AsDestinationRef(subscriber)
		d.CACerts = eventshub.GetCaCerts(ctx)
		trigger.Install(triggerName, trigger.WithBrokerName(brokerName), trigger.WithSubscriberFromDestination(d), trigger.WithFilter(map[string]string{
			"type": event.Type(),
		}))(ctx, t)
	})

	f.Setup("trigger is ready", trigger.IsReady(triggerName))

	f.Setup("install the trigger and specify the CA cert of the destination", func(ctx context.Context, t feature.T) {
		d := service.AsDestinationRef(reply)
		d.CACerts = eventshub.GetCaCerts(ctx)
		trigger.Install(helperTriggerName, trigger.WithBrokerName(brokerName), trigger.WithSubscriberFromDestination(d), trigger.WithFilter(map[string]string{
			"type": replyEventType,
		}))(ctx, t)
	})

	// Send events after data plane is ready.
	f.Requirement("install source", eventshub.Install(source,
		eventshub.StartSenderToResourceTLS(broker.GVR(), brokerName, nil),
		eventshub.InputEvent(event),
	))

	// Assert events ended up where we expected.
	f.Stable("broker with reply").
		Must("deliver event to reply sink", eventassert.OnStore(reply).MatchReceivedEvent(test.HasSource(replyEventSource)).AtLeast(1))

	return f
}
