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

func Success(sinkType integrationsink.SinkType) *feature.Feature {
	f := feature.NewFeature()

	integrationSink := feature.MakeRandomK8sName("integrationsink")
	source := feature.MakeRandomK8sName("source")

	f.Setup("install integration sink", integrationsink.InstallByType(integrationSink, sinkType))

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

	// Sink-specific setup, assertions, and teardown
	switch sinkType {
	case integrationsink.SinkTypeS3:
		f.Setup("cleanup S3 bucket before test", integrationsink.CleanupS3())
		f.Assert("verify S3 object count", integrationsink.AssertS3ObjectCount(2))
		f.Teardown("cleanup S3 bucket after test", integrationsink.CleanupS3())

	case integrationsink.SinkTypeSQS:
		f.Setup("cleanup SQS queue before test", integrationsink.CleanupSQS())
		f.Assert("verify SQS messages", integrationsink.AssertSQSMessages("hello", 2))
		f.Teardown("cleanup SQS queue after test", integrationsink.CleanupSQS())

	case integrationsink.SinkTypeSNS:
		f.Setup("cleanup SNS verification queue before test", integrationsink.CleanupSNSVerificationQueue())
		f.Assert("verify SNS messages", integrationsink.AssertSNSMessages("hello", 2))
		f.Teardown("cleanup SNS verification queue after test", integrationsink.CleanupSNSVerificationQueue())
	}

	return f
}

func SuccessTLS(sinkType integrationsink.SinkType) *feature.Feature {
	f := feature.NewFeature()

	//	sink := feature.MakeRandomK8sName("sink")
	integrationSink := feature.MakeRandomK8sName("integrationsink")
	source := feature.MakeRandomK8sName("source")

	//sinkURL := &apis.URL{Scheme: "http", Host: sink}

	event := cetest.FullEvent()
	event.SetID(uuid.NewString())

	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())
	f.Prerequisite("should not run when Istio is enabled", featureflags.IstioDisabled())

	f.Setup("install integration sink", integrationsink.InstallByType(integrationSink, sinkType))

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
