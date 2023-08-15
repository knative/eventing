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

package eventtype_autocreation

import (
	"context"

	"github.com/cloudevents/sdk-go/v2/test"
	"k8s.io/apimachinery/pkg/util/sets"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/pingsource"
	"knative.dev/eventing/test/rekt/resources/trigger"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"
)

// SendsEventsWithEventTypes tests pingsource to a ready broker.
func SendsEventsFromPingSourceWithEventTypes() *feature.Feature {
	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	via := feature.MakeRandomK8sName("via")

	f := new(feature.Feature)

	f.Setup("enable eventtype auto-creation", ApplyEventTypeConfigMap())

	//Install the broker
	brokerName := feature.MakeRandomK8sName("broker")
	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("broker is ready", broker.IsReady(brokerName))
	f.Setup("broker is addressable", broker.IsAddressable(brokerName))
	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install trigger", trigger.Install(via, brokerName, trigger.WithSubscriber(service.AsKReference(sink), "")))
	f.Setup("trigger goes ready", trigger.IsReady(via))

	f.Requirement("install pingsource", func(ctx context.Context, t feature.T) {
		brokeruri, err := broker.Address(ctx, brokerName)
		if err != nil {
			t.Error("failed to get address of broker", err)
		}
		cfg := []manifest.CfgFn{
			pingsource.WithSink(&duckv1.Destination{URI: brokeruri.URL, CACerts: brokeruri.CACerts}),
			pingsource.WithData("text/plain", "hello, world!"),
		}
		pingsource.Install(source, cfg...)(ctx, t)
	})
	f.Requirement("PingSource goes ready", pingsource.IsReady(source))

	expectedCeTypes := sets.NewString(sourcesv1.PingSourceEventType)

	f.Stable("pingsource as event source").
		Must("delivers events on broker with URI", assert.OnStore(sink).MatchEvent(
			test.HasType("dev.knative.sources.ping")).AtLeast(1)).
		Must("PingSource test eventtypes match", WaitForEventType(
			EventType(AssertPresent(expectedCeTypes))))

	return f
}
