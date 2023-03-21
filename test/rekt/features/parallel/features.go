/*
Copyright 2022 The Knative Authors

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

package parallel

import (
	"strconv"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/test"

	"knative.dev/eventing/test/rekt/resources/channel_template"
	"knative.dev/eventing/test/rekt/resources/parallel"

	"knative.dev/reconciler-test/pkg/eventshub"
	eventasssert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"
)

func ParallelWithTwoBranches() *feature.Feature {
	f := feature.NewFeatureNamed("Parallel test.")

	parallelName := feature.MakeRandomK8sName("parallel1")
	source := feature.MakeRandomK8sName("source1")
	sink := feature.MakeRandomK8sName("sink1")

	// Construct cloudevent message
	event := cloudevents.NewEvent()
	eventID := "CE0.3"
	eventSource := "http://sender.svc/"
	event.SetID(eventID)
	event.SetType("myevent")
	event.SetSource(eventSource)
	eventBody := `{"msg":"test msg"}`
	event.SetData(cloudevents.ApplicationJSON, []byte(eventBody))

	channelTemplate := channel_template.ImmemoryChannelTemplate()

	cfg := []manifest.CfgFn{
		parallel.WithReply(service.AsKReference(sink), ""),
		parallel.WithChannelTemplate(channelTemplate),
	}

	// Construct two branches
	branch1Num := 0
	branch2Num := 1
	subscriber1 := feature.MakeRandomK8sName("subscriber" + strconv.Itoa(branch1Num))
	subscriber2 := feature.MakeRandomK8sName("subscriber" + strconv.Itoa(branch2Num))
	filter1 := feature.MakeRandomK8sName("filter" + strconv.Itoa(branch1Num))
	filter2 := feature.MakeRandomK8sName("filter" + strconv.Itoa(branch2Num))

	f.Setup("install sink", eventshub.Install(sink, eventshub.StartReceiver))
	f.Setup("install subscriber1", eventshub.Install(subscriber1, eventshub.ReplyWithAppendedData("appended data"), eventshub.StartReceiver))
	f.Setup("install subscriber2", eventshub.Install(subscriber2, eventshub.ReplyWithAppendedData("appended data"), eventshub.StartReceiver))
	// Construct branch1 has valid filter but branch2 filter is invalid
	// filter1 simulate the Filter to reply with the filtered event. filter2 has no reply
	f.Setup("install filter1", eventshub.Install(filter1, eventshub.ReplyWithTransformedEvent(event.Type(), eventSource, eventBody), eventshub.StartReceiver))
	f.Setup("install filter2", eventshub.Install(filter2, eventshub.StartReceiver))
	cfg = append(cfg,
		parallel.WithSubscriberAt(branch1Num, service.AsKReference(subscriber1), ""),
		parallel.WithSubscriberAt(branch2Num, service.AsKReference(subscriber2), ""),
		parallel.WithFilterAt(branch1Num, service.AsKReference(filter1), ""),
		parallel.WithFilterAt(branch2Num, service.AsKReference(filter2), ""),
		parallel.WithReplyAt(branch1Num, nil, ""),
		parallel.WithReplyAt(branch2Num, nil, ""),
	)
	// Install a Parallel with two branches
	f.Setup("install Parallel", parallel.Install(parallelName, cfg...))
	f.Requirement("Parallel goes ready", parallel.IsReady(parallelName))

	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResource(parallel.GVR(), parallelName),
		eventshub.InputEvent(event),
	))

	f.Stable("test Parallel with two branches, only first one passed").
		Must("deliver event to branch1 with Filter valid and Branch2 with Filter invalid",
			eventasssert.OnStore(sink).MatchEvent(
				test.HasData([]byte("appended data")),
			).Exact(1))

	return f
}
