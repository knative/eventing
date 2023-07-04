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

package sequence

import (
	"context"
	"fmt"

	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/eventing/test/rekt/resources/channel_template"
	"knative.dev/eventing/test/rekt/resources/pingsource"
	"knative.dev/eventing/test/rekt/resources/sequence"

	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"

	"knative.dev/reconciler-test/pkg/eventshub/assert"
)

func SequenceTest(channelTemplate channel_template.ChannelTemplate) *feature.Feature {
	f := feature.NewFeatureNamed("Sequence test.")

	sequenceName := feature.MakeRandomK8sName("sequence")
	source := feature.MakeRandomK8sName("source")
	sink := feature.MakeRandomK8sName("sink")
	// Sequence's steps
	step1 := feature.MakeRandomK8sName("step1")
	step2 := feature.MakeRandomK8sName("step2")
	step3 := feature.MakeRandomK8sName("step3")
	msgAppender1 := "-step1"
	msgAppender2 := "-step2"
	msgAppender3 := "-step3"

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	// Construct steps with appended data
	f.Setup("install subscriber1", eventshub.Install(step1, eventshub.ReplyWithAppendedData(msgAppender1), eventshub.StartReceiver))
	f.Setup("install subscriber2", eventshub.Install(step2, eventshub.ReplyWithAppendedData(msgAppender2), eventshub.StartReceiver))
	f.Setup("install subscriber", eventshub.Install(step3, eventshub.ReplyWithAppendedData(msgAppender3), eventshub.StartReceiver))

	cfg := []manifest.CfgFn{
		sequence.WithReply(service.AsKReference(sink), ""),
		sequence.WithChannelTemplate(channelTemplate),
	}

	cfg = append(cfg,
		sequence.WithStep(service.AsKReference(step1), ""),
		sequence.WithStep(service.AsKReference(step2), ""),
		sequence.WithStep(service.AsKReference(step3), ""),
	)

	// Install a Sequence with three steps
	f.Setup("install Sequence", sequence.Install(sequenceName, cfg...))
	f.Setup("Sequence goes ready", sequence.IsReady(sequenceName))

	eventBody := fmt.Sprintf("TestSequence %s", uuid.New().String())
	// Install PingSource point to sequence Address with eventBody
	f.Requirement("install pingsource", func(ctx context.Context, t feature.T) {
		sequenceUri, err := sequence.Address(ctx, sequenceName)
		if err != nil {
			t.Error("failed to get address of broker", err)
		}
		cfg := []manifest.CfgFn{
			pingsource.WithSink(&duckv1.Destination{URI: sequenceUri.URL, CACerts: sequenceUri.CACerts}),
			pingsource.WithData("text/plain", eventBody),
		}
		pingsource.Install(source, cfg...)(ctx, t)
	})
	f.Requirement("PingSource goes ready", pingsource.IsReady(source))

	// verify the sink receives the correct appended event
	expectedMsg := eventBody
	expectedMsg += msgAppender1
	expectedMsg += msgAppender2
	expectedMsg += msgAppender3

	f.Stable("pingsource as event source").
		Must("delivers events on sequence with URI",
			assert.OnStore(sink).MatchEvent(
				test.HasType("dev.knative.sources.ping"),
				test.HasDataContentType("text/plain"),
				test.HasData([]byte(expectedMsg)),
			).AtLeast(1))

	return f
}
