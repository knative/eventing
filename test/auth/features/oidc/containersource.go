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
	"knative.dev/eventing/test/rekt/resources/containersource"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/resources/service"
)

func SendsEventsWithSinkRefOIDC() *feature.Feature {
	source := feature.MakeRandomK8sName("containersource")
	sink := feature.MakeRandomK8sName("sink")
	sinkAudience := "audience"
	f := feature.NewFeature()

	f.Setup("install sink", eventshub.Install(sink,
		eventshub.OIDCReceiverAudience(sinkAudience),
		eventshub.StartReceiver))

	f.Requirement("install containersource", containersource.Install(source,
		containersource.WithSink(&duckv1.Destination{
			Ref:      service.AsKReference(sink),
			Audience: &sinkAudience,
		})))
	f.Requirement("containersource goes ready", containersource.IsReady(source))

	f.Stable("containersource as event source").
		Must("delivers events",
			assert.OnStore(sink).MatchEvent(test.HasType("dev.knative.eventing.samples.heartbeat")).AtLeast(1))

	return f
}
