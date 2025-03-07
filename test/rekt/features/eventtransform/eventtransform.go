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

package eventtransform

import (
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	eventing "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/test/rekt/features/featureflags"
	"knative.dev/eventing/test/rekt/resources/addressable"
	"knative.dev/eventing/test/rekt/resources/eventtransform"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/resources/service"
)

// JsonataDirect tests that the EventTransform replies with the transformed event back as HTTP response.
func JsonataDirect() *feature.Feature {
	f := feature.NewFeature()

	transformName := feature.MakeRandomK8sName("event-transform")
	source := feature.MakeRandomK8sName("source")

	const (
		reason  = "test-reason"
		message = "test-message"
	)

	event := cloudevents.NewEvent()
	event.SetID(uuid.NewString())
	event.SetSource("my-source")
	event.SetType("my-type")
	event.SetTime(time.Now())
	_ = event.SetData(cloudevents.ApplicationJSON, map[string]interface{}{
		"reason":  reason,
		"message": message,
	})

	f.Setup("Install event transform", eventtransform.Install(transformName, eventtransform.WithSpec(
		eventtransform.WithJsonata(eventing.JsonataEventTransformationSpec{Expression: `
{
    "specversion": "1.0",
    "id": id,
    "type": "transformed-event",
    "source": source,
    "reason": data.reason,
    "message": data.message,
	"kind": "input",
    "data": $
}
`}),
	)))
	f.Setup("event transform is addressable", eventtransform.IsAddressable(transformName))
	f.Setup("event transform is ready", eventtransform.IsReady(transformName))

	f.Requirement("install source", eventshub.Install(source,
		eventshub.InputEvent(event),
		eventshub.StartSenderToResource(eventtransform.GVR(), transformName)),
	)

	f.Assert("expected sent event", assert.OnStore(source).
		MatchSentEvent(
			cetest.HasId(event.ID()),
			cetest.HasSource(event.Source()),
			cetest.HasType(event.Type()),
			cetest.DataContains(reason),
			cetest.DataContains(message),
		).Exact(1),
	)

	f.Assert("expected transformed event as response", assert.OnStore(source).
		Match(PositiveStatusCode).
		MatchResponseEvent(
			cetest.HasId(event.ID()),
			cetest.HasSource(event.Source()),
			cetest.HasType("transformed-event"),
			cetest.HasExtension("reason", reason),
			cetest.HasExtension("message", message),
			cetest.DataContains(event.Type()),
			cetest.DataContains(event.Source()),
			cetest.DataContains(event.ID()),
			cetest.DataContains(reason),
			cetest.DataContains(message),
		).Exact(1),
	)

	return f
}

// JsonataSink tests that the EventTransform forwards the transformed event to the sink.
func JsonataSink() *feature.Feature {
	f := feature.NewFeature()

	transformName := feature.MakeRandomK8sName("event-transform")
	sink := feature.MakeRandomK8sName("sink")
	source := feature.MakeRandomK8sName("source")

	const (
		reason  = "test-reason"
		message = "test-message"
	)

	event := cloudevents.NewEvent()
	event.SetID(uuid.NewString())
	event.SetSource("my-source")
	event.SetType("my-type")
	event.SetTime(time.Now())
	_ = event.SetData(cloudevents.ApplicationJSON, map[string]interface{}{
		"reason":  reason,
		"message": message,
	})

	f.Setup("Install event transform", eventtransform.Install(transformName, eventtransform.WithSpec(
		eventtransform.WithSink(service.AsDestinationRef(sink)),
		eventtransform.WithJsonata(eventing.JsonataEventTransformationSpec{Expression: `
{
    "specversion": "1.0",
    "id": id,
    "type": "transformed-event",
    "source": source,
    "reason": data.reason,
    "message": data.message,
    "data": $
}
`}),
	)))
	f.Setup("event transform is addressable", eventtransform.IsAddressable(transformName))
	f.Setup("event transform is ready", eventtransform.IsReady(transformName))
	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))

	f.Requirement("install source", eventshub.Install(source,
		eventshub.InputEvent(event),
		eventshub.StartSenderToResource(eventtransform.GVR(), transformName)),
	)

	f.Assert("expected transformed event", assert.OnStore(sink).
		MatchReceivedEvent(
			cetest.HasId(event.ID()),
			cetest.HasSource(event.Source()),
			cetest.HasType("transformed-event"),
			cetest.HasExtension("reason", reason),
			cetest.HasExtension("message", message),
			cetest.DataContains(event.Type()),
			cetest.DataContains(event.Source()),
			cetest.DataContains(event.ID()),
			cetest.DataContains(reason),
			cetest.DataContains(message),
		).Exact(1),
	)

	f.Assert("source received a 2xx status code", assert.OnStore(source).
		Match(
			assert.MatchKind(eventshub.EventResponse),
			PositiveStatusCode,
		).
		Exact(1),
	)

	return f
}

func JsonataSinkReplyTransform() *feature.Feature {
	f := feature.NewFeature()

	transformName := feature.MakeRandomK8sName("event-transform")
	sink := feature.MakeRandomK8sName("sink")
	source := feature.MakeRandomK8sName("source")

	const (
		reason  = "test-reason"
		message = "test-message"
	)

	event := cloudevents.NewEvent()
	event.SetID(uuid.NewString())
	event.SetSource("my-source")
	event.SetType("my-type")
	event.SetTime(time.Now())
	_ = event.SetData(cloudevents.ApplicationJSON, map[string]interface{}{
		"reason":  reason,
		"message": message,
	})

	f.Setup("Install event transform", eventtransform.Install(transformName, eventtransform.WithSpec(
		eventtransform.WithSink(service.AsDestinationRef(sink)),
		eventtransform.WithJsonata(eventing.JsonataEventTransformationSpec{Expression: `
{
    "specversion": "1.0",
    "id": id,
    "type": "transformed-event",
    "source": source,
    "reason": data.reason,
    "message": data.message,
	"kind": "input",
    "data": $
}
`}),
		eventtransform.WithReplyJsonata(eventing.JsonataEventTransformationSpec{Expression: `
{
    "specversion": "1.0",
    "id": id,
    "type": type,
    "source": source,
    "reason": reason,
	"kind": "transformed",
    "data": $
}
`}),
	)))
	f.Setup("event transform is addressable", eventtransform.IsAddressable(transformName))
	f.Setup("event transform is ready", eventtransform.IsReady(transformName))

	const (
		replyEventType   = "reply-event-type"
		replyEventSource = "reply-event-source"
	)

	f.Setup("install sink", eventshub.Install(sink,
		eventshub.ReplyWithTransformedEvent(replyEventType, replyEventSource, `{"reason": "reply-reason"}`),
		eventshub.StartReceiver,
	))

	f.Requirement("install source", eventshub.Install(source,
		eventshub.InputEvent(event),
		eventshub.StartSenderToResource(eventtransform.GVR(), transformName)),
	)

	f.Assert("expected transformed event to the sink", assert.OnStore(sink).
		MatchReceivedEvent(
			cetest.HasId(event.ID()),
			cetest.HasSource(event.Source()),
			cetest.HasType("transformed-event"),
			cetest.HasExtension("reason", reason),
			cetest.HasExtension("message", message),
			cetest.DataContains(event.Type()),
			cetest.DataContains(event.Source()),
			cetest.DataContains(event.ID()),
			cetest.DataContains(reason),
			cetest.DataContains(message),
		).Exact(1),
	)

	f.Assert("expected response transformed event to the source", assert.OnStore(source).
		Match(PositiveStatusCode).
		MatchResponseEvent(
			cetest.HasSource(replyEventSource),
			cetest.HasType(replyEventType),
			cetest.HasExtension("reason", reason),
			cetest.DataContains(replyEventType),
			cetest.DataContains(replyEventSource),
		).Exact(1),
	)

	return f
}

func JsonataDirectTLS() *feature.Feature {
	f := feature.NewFeature()

	transformName := feature.MakeRandomK8sName("event-transform")
	source := feature.MakeRandomK8sName("source")

	const (
		reason  = "test-reason"
		message = "test-message"
	)

	event := cloudevents.NewEvent()
	event.SetID(uuid.NewString())
	event.SetSource("my-source")
	event.SetType("my-type")
	event.SetTime(time.Now())
	_ = event.SetData(cloudevents.ApplicationJSON, map[string]interface{}{
		"reason":  reason,
		"message": message,
	})

	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())

	f.Setup("Install event transform", eventtransform.Install(transformName, eventtransform.WithSpec(
		eventtransform.WithJsonata(eventing.JsonataEventTransformationSpec{Expression: `
{
    "specversion": "1.0",
    "id": id,
    "type": "transformed-event",
    "source": source,
    "reason": data.reason,
    "message": data.message,
	"kind": "input",
    "data": $
}
`}),
	)))
	f.Setup("event transform is addressable", eventtransform.IsAddressable(transformName))
	f.Setup("event transform is ready", eventtransform.IsReady(transformName))
	f.Setup("event transform has HTTPS address", eventtransform.ValidateAddress(transformName, addressable.AssertHTTPSAddress))

	f.Requirement("install source", eventshub.Install(source,
		eventshub.InputEvent(event),
		eventshub.StartSenderToResourceTLS(eventtransform.GVR(), transformName, nil)),
	)

	f.Assert("expected sent event", assert.OnStore(source).
		MatchSentEvent(
			cetest.HasId(event.ID()),
			cetest.HasSource(event.Source()),
			cetest.HasType(event.Type()),
			cetest.DataContains(reason),
			cetest.DataContains(message),
		).Exact(1),
	)

	f.Assert("expected transformed event as response", assert.OnStore(source).
		Match(PositiveStatusCode).
		MatchResponseEvent(
			cetest.HasId(event.ID()),
			cetest.HasSource(event.Source()),
			cetest.HasType("transformed-event"),
			cetest.HasExtension("reason", reason),
			cetest.HasExtension("message", message),
			cetest.DataContains(event.Type()),
			cetest.DataContains(event.Source()),
			cetest.DataContains(event.ID()),
			cetest.DataContains(reason),
			cetest.DataContains(message),
		).Exact(1),
	)

	return f
}

// JsonataSinkTLS tests that the EventTransform forwards the transformed event to the sink.
func JsonataSinkTLS() *feature.Feature {
	f := feature.NewFeature()

	transformName := feature.MakeRandomK8sName("event-transform")
	sink := feature.MakeRandomK8sName("sink")
	source := feature.MakeRandomK8sName("source")

	const (
		reason  = "test-reason"
		message = "test-message"
	)

	event := cloudevents.NewEvent()
	event.SetID(uuid.NewString())
	event.SetSource("my-source")
	event.SetType("my-type")
	event.SetTime(time.Now())
	_ = event.SetData(cloudevents.ApplicationJSON, map[string]interface{}{
		"reason":  reason,
		"message": message,
	})

	f.Prerequisite("transport encryption is strict", featureflags.TransportEncryptionStrict())

	f.Setup("Install event transform", eventtransform.Install(transformName, eventtransform.WithSpec(
		eventtransform.WithSink(&duckv1.Destination{URI: apis.HTTPS(sink)}),
		eventtransform.WithJsonata(eventing.JsonataEventTransformationSpec{Expression: `
{
    "specversion": "1.0",
    "id": id,
    "type": "transformed-event",
    "source": source,
    "reason": data.reason,
    "message": data.message,
    "data": $
}
`}),
	)))
	f.Setup("event transform is addressable", eventtransform.IsAddressable(transformName))
	f.Setup("event transform is ready", eventtransform.IsReady(transformName))
	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiverTLS))
	f.Setup("event transform has HTTPS address", eventtransform.ValidateAddress(transformName, addressable.AssertHTTPSAddress))

	f.Requirement("install source", eventshub.Install(source,
		eventshub.InputEvent(event),
		eventshub.StartSenderToResourceTLS(eventtransform.GVR(), transformName, nil)),
	)

	f.Assert("expected transformed event", assert.OnStore(sink).
		MatchReceivedEvent(
			cetest.HasId(event.ID()),
			cetest.HasSource(event.Source()),
			cetest.HasType("transformed-event"),
			cetest.HasExtension("reason", reason),
			cetest.HasExtension("message", message),
			cetest.DataContains(event.Type()),
			cetest.DataContains(event.Source()),
			cetest.DataContains(event.ID()),
			cetest.DataContains(reason),
			cetest.DataContains(message),
		).Exact(1),
	)

	f.Assert("source received a 2xx status code", assert.OnStore(source).
		Match(
			assert.MatchKind(eventshub.EventResponse),
			PositiveStatusCode,
		).
		Exact(1),
	)

	return f
}

func PositiveStatusCode(info eventshub.EventInfo) error {
	if info.StatusCode < 200 || info.StatusCode >= 300 {
		return fmt.Errorf("expected 2xx status code, got %d", info.StatusCode)
	}
	return nil
}
