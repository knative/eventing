/*
Copyright 2022 The Knative Authors

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

package trigger

import (
	"context"
	"fmt"

	"github.com/cloudevents/sdk-go/v2/test"
	"k8s.io/utils/pointer"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/network"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/knative"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"

	"knative.dev/reconciler-test/pkg/eventshub/assert"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/eventingtls/eventingtlstesting"
	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/configmap"
	"knative.dev/eventing/test/rekt/resources/pingsource"
	"knative.dev/eventing/test/rekt/resources/trigger"
)

// This test is for avoiding regressions on the trigger dependency annotation functionality.
func TriggerDependencyAnnotation() *feature.Feature {
	sink := feature.MakeRandomK8sName("sink")
	triggerName := feature.MakeRandomK8sName("triggerName")

	f := new(feature.Feature)

	//Install the broker
	brokerName := feature.MakeRandomK8sName("broker")
	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("broker is ready", broker.IsReady(brokerName))
	f.Setup("broker is addressable", broker.IsAddressable(brokerName))

	psourcename := "test-ping-source-annotation"
	dependencyAnnotation := `{"kind":"PingSource","name":"test-ping-source-annotation","apiVersion":"sources.knative.dev/v1"}`
	annotations := map[string]interface{}{
		"knative.dev/dependency": dependencyAnnotation,
	}

	// Add the annotation to trigger and point the Trigger subscriber to the sink svc.
	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	cfg := []manifest.CfgFn{
		trigger.WithSubscriber(service.AsKReference(sink), ""),
		trigger.WithAnnotations(annotations),
		trigger.WithBrokerName(brokerName),
	}
	// Install the trigger
	f.Setup("install trigger", trigger.Install(triggerName, cfg...))

	// trigger is not ready since the pingsource dependency is not installed yet
	f.Setup("trigger is not ready before pingsource dependency exists", trigger.IsNotReady(triggerName))

	// verify that the trigger has the DependencyDoesNotExist condition
	f.Setup("trigger has DependencyDoesNotExist condition", trigger.DependencyDoesNotExist(triggerName))

	// trigger won't go ready until after the pingsource exists, because of the dependency annotation
	f.Requirement("trigger goes ready", trigger.IsReady(triggerName))

	f.Requirement("install pingsource", func(ctx context.Context, t feature.T) {
		brokeruri, err := broker.Address(ctx, brokerName)
		if err != nil {
			t.Error("failed to get address of broker", err)
		}
		cfg := []manifest.CfgFn{
			pingsource.WithSchedule("*/1 * * * *"),
			pingsource.WithSink(&duckv1.Destination{URI: brokeruri.URL, CACerts: brokeruri.CACerts}),
			pingsource.WithData("text/plain", "Test trigger-annotation"),
		}
		pingsource.Install(psourcename, cfg...)(ctx, t)
	})
	f.Requirement("PingSource goes ready", pingsource.IsReady(psourcename))

	f.Stable("pingsource as event source to test trigger with annotations").
		Must("delivers events on broker with URI", assert.OnStore(sink).MatchEvent(
			test.HasType("dev.knative.sources.ping"),
			test.DataContains("Test trigger-annotation"),
		).AtLeast(1))

	return f
}

func TriggerSupportsDeliveryFormat() *feature.FeatureSet {
	return &feature.FeatureSet{
		Name:     "Trigger supports delivery format",
		Features: []*feature.Feature{triggerWithDispatcherFormat("json"), triggerWithDispatcherFormat("binary")},
	}
}

func triggerWithDispatcherFormat(format string) *feature.Feature {
	f := feature.NewFeatureNamed(fmt.Sprintf("Trigger supports sending with %s delivery format", format))

	brokerName := feature.MakeRandomK8sName("broker")
	sourceName := feature.MakeRandomK8sName("source")
	sinkName := feature.MakeRandomK8sName("sink")
	triggerName := feature.MakeRandomK8sName("trigger")
	eventToSend := test.FullEvent()

	f.Setup("Install Broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("Broker is ready", broker.IsReady(brokerName))
	f.Setup("Broker is addressable", broker.IsAddressable(brokerName))

	f.Setup("Install Sink", eventshub.Install(sinkName, eventshub.VerifyEventFormat(format), eventshub.StartReceiver))

	f.Setup("Install trigger", trigger.Install(triggerName, trigger.WithBrokerName(brokerName), trigger.WithFormat(format), trigger.WithSubscriber(service.AsKReference(sinkName), "")))
	f.Setup("Trigger is ready", trigger.IsReady(triggerName))

	f.Requirement("Install source", eventshub.Install(sourceName, eventshub.InputEvent(eventToSend), eventshub.StartSenderToResource(broker.GVR(), brokerName)))

	f.Alpha("trigger").
		Must("dispatch event with correct format", assert.OnStore(sinkName).MatchReceivedEvent(test.HasId(eventToSend.ID())).AtLeast(1))

	return f
}

func TriggerWithTLSSubscriber() *feature.Feature {
	f := feature.NewFeatureNamed("Trigger with TLS subscriber")

	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	brokerName := feature.MakeRandomK8sName("broker")
	sourceName := feature.MakeRandomK8sName("source")
	sinkName := feature.MakeRandomK8sName("sink")
	triggerName := feature.MakeRandomK8sName("trigger")
	dlsName := feature.MakeRandomK8sName("dls")
	dlsTriggerName := feature.MakeRandomK8sName("dls-trigger")

	eventToSend := test.FullEvent()

	// Install Broker
	f.Setup("Install Broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("Broker is ready", broker.IsReady(brokerName))
	f.Setup("Broker is addressable", broker.IsAddressable(brokerName))

	// Install Sink
	f.Setup("Install Sink", eventshub.Install(sinkName, eventshub.StartReceiverTLS))
	f.Setup("Install dead letter sink service", eventshub.Install(dlsName, eventshub.StartReceiverTLS))

	// Install Trigger
	f.Setup("Install trigger", func(ctx context.Context, t feature.T) {
		subscriber := service.AsDestinationRef(sinkName)
		subscriber.CACerts = eventshub.GetCaCerts(ctx)

		trigger.Install(triggerName, trigger.WithBrokerName(brokerName),
			trigger.WithSubscriberFromDestination(subscriber))(ctx, t)
	})
	f.Setup("Wait for Trigger to become ready", trigger.IsReady(triggerName))

	f.Setup("Install failing trigger", func(ctx context.Context, t feature.T) {
		dls := service.AsDestinationRef(dlsName)
		dls.CACerts = eventshub.GetCaCerts(ctx)

		linear := eventingduckv1.BackoffPolicyLinear
		trigger.Install(dlsTriggerName, trigger.WithBrokerName(brokerName),
			trigger.WithRetry(2, &linear, pointer.String("PT1S")),
			trigger.WithDeadLetterSinkFromDestination(dls),
			trigger.WithSubscriber(nil, "http://127.0.0.1:2468"))(ctx, t)
	})
	f.Setup("Wait for failing Trigger to become ready", trigger.IsReady(dlsTriggerName))

	// Install Source
	f.Requirement("Install Source", eventshub.Install(
		sourceName,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(eventToSend),
	))

	f.Assert("Trigger delivers events to TLS subscriber", assert.OnStore(sinkName).
		MatchReceivedEvent(test.HasId(eventToSend.ID())).
		AtLeast(1))
	f.Assert("Trigger delivers events to TLS dead letter sink", assert.OnStore(dlsName).
		MatchReceivedEvent(test.HasId(eventToSend.ID())).
		AtLeast(1))

	return f
}

func TriggerWithTLSSubscriberTrustBundle() *feature.Feature {
	f := feature.NewFeatureNamed("Trigger with TLS subscriber - trust bundle")

	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	brokerName := feature.MakeRandomK8sName("broker")
	sourceName := feature.MakeRandomK8sName("source")
	sinkName := feature.MakeRandomK8sName("sink")
	triggerName := feature.MakeRandomK8sName("trigger")
	dlsName := feature.MakeRandomK8sName("dls")
	dlsTriggerName := feature.MakeRandomK8sName("dls-trigger")

	eventToSend := test.FullEvent()

	// Install Broker
	f.Setup("Install Broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("Broker is ready", broker.IsReady(brokerName))
	f.Setup("Broker is addressable", broker.IsAddressable(brokerName))

	// Install Sink
	f.Setup("Install Sink", eventshub.Install(sinkName,
		eventshub.IssuerRef(eventingtlstesting.IssuerKind, eventingtlstesting.IssuerName),
		eventshub.StartReceiverTLS,
	))
	f.Setup("Install dead letter sink service", eventshub.Install(dlsName,
		eventshub.IssuerRef(eventingtlstesting.IssuerKind, eventingtlstesting.IssuerName),
		eventshub.StartReceiverTLS,
	))

	// Install Trigger
	f.Setup("Install trigger", func(ctx context.Context, t feature.T) {
		subscriber := &duckv1.Destination{
			URI: &apis.URL{
				Scheme: "https", // Force using https
				Host:   network.GetServiceHostname(sinkName, environment.FromContext(ctx).Namespace()),
			},
			CACerts: nil, // CA certs are in the trust-bundle
		}

		trigger.Install(triggerName, trigger.WithBrokerName(brokerName),
			trigger.WithSubscriberFromDestination(subscriber))(ctx, t)
	})
	f.Setup("Wait for Trigger to become ready", trigger.IsReady(triggerName))

	f.Setup("Install failing trigger", func(ctx context.Context, t feature.T) {
		dls := &duckv1.Destination{
			URI: &apis.URL{
				Scheme: "https", // Force using https
				Host:   network.GetServiceHostname(dlsName, environment.FromContext(ctx).Namespace()),
			},
			CACerts: nil, // CA certs are in the trust-bundle
		}

		linear := eventingduckv1.BackoffPolicyLinear
		trigger.Install(dlsTriggerName, trigger.WithBrokerName(brokerName),
			trigger.WithRetry(2, &linear, pointer.String("PT1S")),
			trigger.WithDeadLetterSinkFromDestination(dls),
			trigger.WithSubscriber(nil, "http://127.0.0.1:2468"))(ctx, t)
	})
	f.Setup("Wait for failing Trigger to become ready", trigger.IsReady(dlsTriggerName))

	// Install Source
	f.Requirement("Install Source", eventshub.Install(
		sourceName,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(eventToSend),
	))

	f.Assert("Trigger delivers events to TLS subscriber", assert.OnStore(sinkName).
		MatchEvent(test.HasId(eventToSend.ID())).
		Match(assert.MatchKind(eventshub.EventReceived)).
		AtLeast(1))
	f.Assert("Trigger delivers events to TLS dead letter sink", assert.OnStore(dlsName).
		MatchEvent(test.HasId(eventToSend.ID())).
		Match(assert.MatchKind(eventshub.EventReceived)).
		AtLeast(1))

	return f
}

func TriggerWithTLSSubscriberWithAdditionalCATrustBundles() *feature.Feature {
	f := feature.NewFeatureNamed("Trigger with TLS subscriber and additional trust bundle")

	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	brokerName := feature.MakeRandomK8sName("broker")
	sourceName := feature.MakeRandomK8sName("source")
	sinkName := feature.MakeRandomK8sName("sink")
	triggerName := feature.MakeRandomK8sName("trigger")
	dlsName := feature.MakeRandomK8sName("dls")
	dlsTriggerName := feature.MakeRandomK8sName("dls-trigger")
	trustBundle := feature.MakeRandomK8sName("trust-bundle")

	eventToSend := test.FullEvent()

	// Install Broker
	f.Setup("Install Broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("Broker is ready", broker.IsReady(brokerName))
	f.Setup("Broker is addressable", broker.IsAddressable(brokerName))

	// Install Sink
	f.Setup("Install Sink", eventshub.Install(sinkName, eventshub.StartReceiverTLS))
	f.Setup("Install dead letter sink service", eventshub.Install(dlsName, eventshub.StartReceiverTLS))

	f.Setup("Add trust bundle to system namespace", func(ctx context.Context, t feature.T) {

		configmap.Install(trustBundle, knative.KnativeNamespaceFromContext(ctx),
			configmap.WithLabels(map[string]string{"networking.knative.dev/trust-bundle": "true"}),
			configmap.WithData("ca.crt", *eventshub.GetCaCerts(ctx)),
		)(ctx, t)
	})

	// Install Trigger
	f.Setup("Install trigger", func(ctx context.Context, t feature.T) {
		subscriber := &duckv1.Destination{
			URI: &apis.URL{
				Scheme: "https", // Force using https
				Host:   network.GetServiceHostname(sinkName, environment.FromContext(ctx).Namespace()),
			},
			CACerts: nil, // CA certs are in the new trust-bundle
		}

		trigger.Install(triggerName, trigger.WithBrokerName(brokerName),
			trigger.WithSubscriberFromDestination(subscriber))(ctx, t)
	})
	f.Setup("Wait for Trigger to become ready", trigger.IsReady(triggerName))

	f.Setup("Install failing trigger", func(ctx context.Context, t feature.T) {
		dls := &duckv1.Destination{
			URI: &apis.URL{
				Scheme: "https", // Force using https
				Host:   network.GetServiceHostname(dlsName, environment.FromContext(ctx).Namespace()),
			},
			CACerts: nil, // CA certs are in the new trust-bundle
		}

		linear := eventingduckv1.BackoffPolicyLinear
		trigger.Install(dlsTriggerName, trigger.WithBrokerName(brokerName),
			trigger.WithRetry(2, &linear, pointer.String("PT1S")),
			trigger.WithDeadLetterSinkFromDestination(dls),
			trigger.WithSubscriber(nil, "http://127.0.0.1:2468"))(ctx, t)
	})
	f.Setup("Wait for failing Trigger to become ready", trigger.IsReady(dlsTriggerName))

	// Install Source
	f.Requirement("Install Source", eventshub.Install(
		sourceName,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(eventToSend),
	))

	f.Assert("Trigger delivers events to TLS subscriber", assert.OnStore(sinkName).
		MatchReceivedEvent(test.HasId(eventToSend.ID())).
		AtLeast(1))
	f.Assert("Trigger delivers events to TLS dead letter sink", assert.OnStore(dlsName).
		MatchReceivedEvent(test.HasId(eventToSend.ID())).
		AtLeast(1))

	return f
}
