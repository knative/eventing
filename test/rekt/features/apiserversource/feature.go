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

package apiserversource

import (
	"context"
	"knative.dev/eventing/test/rekt/resources/apiserversource"
	"knative.dev/eventing/test/rekt/resources/broker"

	"github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/resources/service"

	"knative.dev/reconciler-test/pkg/eventshub/assert"
	eventassert "knative.dev/reconciler-test/pkg/eventshub/assert"

	"knative.dev/eventing/test/rekt/features/source"
)

func SendsEventsWithBrokerAsSink() *feature.Feature {
	src := feature.MakeRandomK8sName("apiserversource")
	brokerName := feature.MakeRandomK8sName("broker")
	f := feature.NewFeature()

	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("broker is ready", broker.IsReady(brokerName))
	f.Setup("broker is addressable", broker.IsAddressable(brokerName))

	f.Setup("install broker as the sink", eventshub.Install(brokerName, eventshub.StartReceiverTLS))

	f.Requirement("install apiserversource", func(ctx context.Context, t feature.T) {
		d := service.AsDestinationRef(brokerName)
		d.CACerts = eventshub.GetCaCerts(ctx)

		apiserversource.Install(src, apiserversource.WithSink(d))(ctx, t)
	})
	f.Requirement("apiserversource goes ready", apiserversource.IsReady(src))

	f.Stable("apiserversource as event source").
		Must("delivers events", assert.OnStore(brokerName).
			Match(eventassert.MatchKind(eventshub.EventReceived)).
			MatchEvent(test.HasType("dev.knative.apiserver.resource.add")).
			AtLeast(1)).
		Must("Set sinkURI to HTTPS endpoint", source.ExpectHTTPSSink(apiserversource.Gvr(), src)).
		Must("Set sinkCACerts to non empty CA certs", source.ExpectCACerts(apiserversource.Gvr(), src))

	return f
}
