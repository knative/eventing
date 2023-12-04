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

package oidc

import (
	"github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/trigger"
	v1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/eventshub"
	eventassert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/resources/service"
)

func BrokerSendEventWithOIDCToken() *feature.Feature {
	f := feature.NewFeatureNamed("Broker supports flow with OIDC tokens")

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
		eventshub.OIDCReceiverAudience(sinkAudience),
		eventshub.StartReceiver))

	// Install the trigger and Point the Trigger subscriber to the sink svc.
	f.Setup("install trigger", trigger.Install(
		triggerName,
		brokerName,
		trigger.WithSubscriberFromDestination(&v1.Destination{
			Ref:      service.AsKReference(sink),
			Audience: &sinkAudience,
		}),
	))
	f.Setup("trigger goes ready", trigger.IsReady(triggerName))

	// Send event
	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
	))

	f.Alpha("Broker").
		Must("handles event with valid OIDC token", eventassert.OnStore(sink).MatchReceivedEvent(test.HasId(event.ID())).Exact(1))

	return f
}
