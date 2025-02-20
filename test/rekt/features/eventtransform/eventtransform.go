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
	cloudevents "github.com/cloudevents/sdk-go/v2"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	eventing "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/test/rekt/resources/eventtransform"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/resources/service"
	"time"
)

func Sink() *feature.Feature {
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
		MatchEvent(
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
		).AtLeast(1),
	)

	return f
}
