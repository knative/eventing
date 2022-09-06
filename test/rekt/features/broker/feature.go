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

package broker

import (
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"knative.dev/reconciler-test/pkg/eventshub"
	eventasssert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/resources/svc"

	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/trigger"
	"knative.dev/reconciler-test/pkg/manifest"
)

func BrokerPreferHeaderCheck() *feature.Feature {
	f := feature.NewFeatureNamed("Channel PreferHeader Check")

	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	via := feature.MakeRandomK8sName("via")

	eventSource := "source1"
	eventType := "type1"
	eventBody := `{"msg":"test msg"}`
	event := cloudevents.NewEvent()
	event.SetID(uuid.New().String())
	event.SetType(eventType)
	event.SetSource(eventSource)
	event.SetData(cloudevents.ApplicationJSON, []byte(eventBody))

	//Install the broker
	brokerName := feature.MakeRandomK8sName("broker")
	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Requirement("broker is ready", broker.IsReady(brokerName))
	f.Requirement("broker is addressable", broker.IsAddressable(brokerName))

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	// Point the Trigger subscriber to the sink svc.
	cfg := []manifest.CfgFn{trigger.WithSubscriber(svc.AsKReference(sink), "")}

	// Install the trigger
	f.Setup("install trigger", trigger.Install(via, brokerName, cfg...))
	f.Setup("trigger goes ready", trigger.IsReady(via))

	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
	))

	f.Stable("test message without explicit prefer header should have the header").
		Must("delivers events",
			eventasssert.OnStore(sink).Match(
				eventasssert.HasAdditionalHeader("Prefer", "reply"),
			).AtLeast(1))

	return f
}
