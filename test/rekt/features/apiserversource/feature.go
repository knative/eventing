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
	"github.com/cloudevents/sdk-go/v2/test"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/test/rekt/resources/apiserversource"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/trigger"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/eventshub"
	eventasssert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

func SendsEventsWithBrokerAsSinkTLS() *feature.Feature {
	src := feature.MakeRandomK8sName("apiserversource")
	brokerName := feature.MakeRandomK8sName("broker")
	sacmName := feature.MakeRandomK8sName("apiserversource")
	sink := feature.MakeRandomK8sName("sink")
	via := feature.MakeRandomK8sName("via")
	f := feature.NewFeature()

	f.Setup("install broker", broker.Install(brokerName, broker.WithEnvConfig()...))
	f.Setup("broker is ready", broker.IsReady(brokerName))
	f.Setup("broker is addressable", broker.IsAddressable(brokerName))
	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiverTLS))

	f.Setup("install trigger", func(ctx context.Context, t feature.T) {
		caCerts := eventshub.GetCaCerts(ctx)
		brokeruri, err := broker.Address(ctx, brokerName)
		if err != nil {
			t.Error("failed to get address of broker", err)
		}
		trigger.Install(via, brokerName, trigger.WithSubscriberFromDestination(&duckv1.Destination{
			URI:     brokeruri.URL,
			CACerts: caCerts,
		}))(ctx, t)
	})

	f.Setup("trigger goes ready", trigger.IsReady(via))
	f.Setup("Create Service Account for ApiServerSource with RBAC for v1.Event resources",
		setupAccountAndRoleForPods(sacmName))

	f.Requirement("install ApiServerSource", func(ctx context.Context, t feature.T) {
		brokeruri, err := broker.Address(ctx, brokerName)
		brokeruri.CACerts = eventshub.GetCaCerts(ctx)
		if err != nil {
			t.Error("failed to get address of broker", err)
		}
		cfg := []manifest.CfgFn{
			apiserversource.WithServiceAccountName(sacmName),
			apiserversource.WithEventMode(v1.ResourceMode),
			apiserversource.WithSink(&duckv1.Destination{URI: brokeruri.URL, CACerts: brokeruri.CACerts}),
			apiserversource.WithResources(v1.APIVersionKindSelector{
				APIVersion: "v1",
				Kind:       "Event",
			}),
		}
		apiserversource.Install(src, cfg...)(ctx, t)
	})
	f.Requirement("ApiServerSource goes ready", apiserversource.IsReady(src))

	f.Stable("ApiServerSource as event source").
		Must("delivers events on the broker sink",
			eventasssert.OnStore(sink).MatchEvent(test.HasType("dev.knative.apiserver.resource.update")).AtLeast(1))

	return f
}
