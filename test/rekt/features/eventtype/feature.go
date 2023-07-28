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

package eventtype

import (
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/eventtype"
	"knative.dev/eventing/test/rekt/resources/trigger"
	"knative.dev/pkg/ptr"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"
)

func eventTypeWithBrokerAsReference(retryNum int32, dropNum uint) *feature.Feature {
	f := feature.NewFeatureNamed("Broker reply with a bad status code to the first n events")

	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	via := feature.MakeRandomK8sName("via")

	eventSource := "source2"
	eventType := "type2"
	eventBody := `{"msg":"DropN"}`
	event := cloudevents.NewEvent()
	event.SetID(uuid.New().String())
	event.SetType(eventType)
	event.SetSource(eventSource)
	event.SetData(cloudevents.ApplicationJSON, []byte(eventBody))

	//Install the broker
	brokerName := feature.MakeRandomK8sName("broker")
	exp := duckv1.BackoffPolicyLinear
	brokerConfig := append(broker.WithEnvConfig(), broker.WithRetry(retryNum, &exp, ptr.String("PT1S")))

	f.Setup("install broker", broker.Install(brokerName, brokerConfig...))
	f.Requirement("broker is ready", broker.IsReady(brokerName))
	f.Requirement("broker is addressable", broker.IsAddressable(brokerName))

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	// Point the Trigger subscriber to the sink svc.
	cfg := []manifest.CfgFn{trigger.WithSubscriber(service.AsKReference(sink), "")}

	// Install the trigger
	f.Setup("install trigger", trigger.Install(via, brokerName, cfg...))
	f.Setup("trigger goes ready", trigger.IsReady(via))

	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResource(broker.GVR(), brokerName),
		eventshub.InputEvent(event),
	))

	// The eventType should be already auto-created. We then need to pop up the eventType in the event registry.
	// We need to validate the reference of the eventType is pointing to the broker.
	f.Stable("ApiServerSource as event source").
		Must("ApiServerSource test eventtypes match",
			eventtype.WaitForEventType(eventtype.AssertReferenceMatch("InMemoryChannel")))

	return f
}
