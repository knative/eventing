/*
Copyright 2021 The Knative Authors

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
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"

	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/svc"
	"knative.dev/eventing/test/rekt/resources/trigger"

	. "github.com/cloudevents/sdk-go/v2/test"
	. "knative.dev/reconciler-test/pkg/eventshub/assert"
)

func BrokerIngressConformanceFeatures(brokerClass string) []*feature.Feature {
	var feats []*feature.Feature
	for _, version := range []string{cloudevents.VersionV03, cloudevents.VersionV1} {
		for _, enc := range []cloudevents.Encoding{cloudevents.EncodingBinary, cloudevents.EncodingStructured} {
			feats = append(feats, brokerIngressConformanceFeature(brokerClass, version, enc))
		}
	}
	feats = append(feats, brokerIngressConformanceBadEvent(brokerClass))
	return feats
}

func brokerIngressConformanceFeature(brokerClass string, version string, enc cloudevents.Encoding) *feature.Feature {
	sourceName := feature.MakeRandomK8sName("source")
	sinkName := feature.MakeRandomK8sName("sink")
	triggerName := feature.MakeRandomK8sName("trigger")
	brokerName := feature.MakeRandomK8sName("broker")

	event := cetest.FullEvent()
	event.SetSpecVersion(version)

	f := new(feature.Feature)
	f.Name = "BrokerIngress" + version + enc.String()

	f.Setup("install sink", eventshub.Install(sinkName, eventshub.StartReceiver))
	f.Setup("install broker", broker.Install(brokerName, broker.WithBrokerClass(brokerClass)))
	f.Setup("install trigger", trigger.Install(triggerName, brokerName, trigger.WithSubscriber(svc.AsRef(sinkName), "")))
	f.Setup("broker is addressable", broker.IsAddressable(brokerName))

	f.Setup("install source", eventshub.Install(
		sourceName,
		eventshub.StartSenderToResource(broker.Gvr(), brokerName),
		eventshub.InputEventWithEncoding(event, enc),
	))

	f.Stable("ingress supports v"+version).
		Must("accept the event", OnStore(sourceName).Match(
			MatchKind(EventResponse),
			MatchStatusCode(202),
		).AtLeast(1)).
		Must("deliver the event",
			OnStore(sinkName).MatchEvent(
				HasId(event.ID()),
				HasSpecVersion(event.SpecVersion()),
			).AtLeast(1))

	return f
}

func brokerIngressConformanceBadEvent(brokerClass string) *feature.Feature {
	sourceName := feature.MakeRandomK8sName("source")
	sinkName := feature.MakeRandomK8sName("sink")
	triggerName := feature.MakeRandomK8sName("trigger")
	brokerName := feature.MakeRandomK8sName("broker")

	eventID := "four-hundred-on-bad-ce"

	f := new(feature.Feature)
	f.Name = "BrokerIngressConformanceBadEvent"

	f.Setup("install sink", eventshub.Install(sinkName, eventshub.StartReceiver))
	f.Setup("install broker", broker.Install(brokerName, broker.WithBrokerClass(brokerClass)))
	f.Setup("install trigger", trigger.Install(triggerName, brokerName, trigger.WithSubscriber(svc.AsRef(sinkName), "")))
	f.Setup("broker is addressable", broker.IsAddressable(brokerName))

	f.Setup("install source", eventshub.Install(sourceName,
		eventshub.StartSenderToResource(broker.Gvr(), brokerName),
		eventshub.InputHeader("ce-specversion", "9000.1"),
		eventshub.InputHeader("ce-type", "sometype"),
		eventshub.InputHeader("ce-source", "400.request.sender.test.knative.dev"),
		eventshub.InputHeader("ce-id", eventID),
		eventshub.InputBody(";la}{kjsdf;oai2095{}{}8234092349807asdfashdf"),
	))

	f.Stable("ingress").
		Must("respond with 400 on bad event", OnStore(sourceName).Match(
			MatchKind(EventResponse),
			MatchStatusCode(400),
		).AtLeast(1)).
		Must("must not propagate bad event",
			OnStore(sinkName).MatchEvent(HasId(eventID)).Not())

	return f
}
