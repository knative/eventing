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

	"github.com/cloudevents/sdk-go/v2/test"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"

	"knative.dev/reconciler-test/pkg/eventshub/assert"

	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/resources/broker"
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
	}

	// Install the trigger
	f.Setup("install trigger", trigger.Install(triggerName, brokerName, cfg...))

	f.Setup("trigger goes ready", trigger.IsReady(triggerName))

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

func TriggerWithTLSSubscriber() *feature.Feature {
	f := feature.NewFeatureNamed("Trigger with TLS subscriber")

	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	brokerName := feature.MakeRandomK8sName("broker")
	sourceName := feature.MakeRandomK8sName("source")
	sinkName := feature.MakeRandomK8sName("sink")
	triggerName := feature.MakeRandomK8sName("trigger")

	eventToSend := test.FullEvent()

	// Install Broker
	f.Setup("Install Broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("Broker is ready", broker.IsReady(brokerName))
	f.Setup("Broker is addressable", broker.IsAddressable(brokerName))

	// Install Sink
	f.Setup("Install Sink", eventshub.Install(sinkName, eventshub.StartReceiverTLS))

	// Install Trigger
	f.Setup("Install trigger", func(ctx context.Context, t feature.T) {
		subscriber := service.AsDestinationRef(sinkName)
		subscriber.CACerts = eventshub.GetCaCerts(ctx)

		trigger.Install(triggerName, brokerName,
			trigger.WithSubscriberFromDestination(subscriber))(ctx, t)
	})
	f.Setup("Wait for Trigger to become ready", trigger.IsReady(triggerName))

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

	return f
}
