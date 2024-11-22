/*
Copyright 2024 The Knative Authors

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

package integrationsink

import (
	"time"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/resources/addressable"
	"knative.dev/eventing/test/rekt/resources/integrationsink"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
)

func Success() *feature.Feature {
	f := feature.NewFeature()

	//	sink := feature.MakeRandomK8sName("sink")
	integrationSink := feature.MakeRandomK8sName("integrationsink")
	source := feature.MakeRandomK8sName("source")

	event := cetest.FullEvent()
	event.SetID(uuid.NewString())

	f.Setup("install integration sink", integrationsink.Install(integrationSink))

	f.Setup("integrationsink is addressable", integrationsink.IsAddressable(integrationSink))
	f.Setup("integrationsink is ready", integrationsink.IsReady(integrationSink))

	f.Requirement("install source for ksink", eventshub.Install(source,
		eventshub.StartSenderToResource(integrationsink.GVR(), integrationSink),
		eventshub.InputEvent(cetest.FullEvent()),
		eventshub.AddSequence,
		eventshub.SendMultipleEvents(2, time.Millisecond)))

	f.Assert("Source sent the event", assert.OnStore(source).
		Match(assert.MatchKind(eventshub.EventResponse)).
		Match(assert.MatchStatusCode(204)).
		AtLeast(1),
	)

	return f
}

func SuccessTLS() *feature.Feature {
	f := feature.NewFeature()

	//	sink := feature.MakeRandomK8sName("sink")
	integrationSink := feature.MakeRandomK8sName("integrationsink")
	source := feature.MakeRandomK8sName("source")

	//sinkURL := &apis.URL{Scheme: "http", Host: sink}

	event := cetest.FullEvent()
	event.SetID(uuid.NewString())

	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install integration sink", integrationsink.Install(integrationSink)) //, integrationsink.WithForwarderJob(sinkURL.String())))

	f.Setup("integrationsink is addressable", integrationsink.IsAddressable(integrationSink))
	f.Setup("integrationsink is ready", integrationsink.IsReady(integrationSink))

	f.Requirement("install source for ksink", eventshub.Install(source,
		eventshub.StartSenderToResource(integrationsink.GVR(), integrationSink),
		eventshub.InputEvent(cetest.FullEvent()),
		eventshub.AddSequence,
		eventshub.SendMultipleEvents(2, time.Millisecond)))

	f.Assert("IntegrationSink has https address", addressable.ValidateAddress(integrationsink.GVR(), integrationSink, addressable.AssertHTTPSAddress))

	f.Assert("Source sent the event", assert.OnStore(source).
		Match(assert.MatchKind(eventshub.EventResponse)).
		Match(assert.MatchStatusCode(204)).
		AtLeast(1),
	)

	return f
}
