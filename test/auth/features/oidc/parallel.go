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
	"context"
	"strconv"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/reconciler/parallel/resources"
	"knative.dev/eventing/test/rekt/resources/addressable"
	"knative.dev/eventing/test/rekt/resources/channel_template"
	"knative.dev/eventing/test/rekt/resources/parallel"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/eventshub"
	eventasssert "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/service"
)

func ParallelHasAudienceOfInputChannel(parallelName, parallelNamespace string, channelGVR schema.GroupVersionResource, channelKind string) *feature.Feature {
	f := feature.NewFeatureNamed("Parallel has audience of input channel")

	f.Setup("Parallel goes ready", parallel.IsReady(parallelName))

	expectedAudience := auth.GetAudience(channelGVR.GroupVersion().WithKind(channelKind), metav1.ObjectMeta{
		Name:      resources.ParallelChannelName(parallelName),
		Namespace: parallelNamespace,
	})

	f.Alpha("Parallel").Must("has audience set", parallel.ValidateAddress(parallelName, addressable.AssertAddressWithAudience(expectedAudience)))

	return f
}

func ParallelWithTwoBranchesOIDC(channelTemplate channel_template.ChannelTemplate) *feature.Feature {
	f := feature.NewFeatureNamed("Parallel test.")

	parallelName := feature.MakeRandomK8sName("parallel1")
	source := feature.MakeRandomK8sName("source1")
	sink := feature.MakeRandomK8sName("sink1")
	sinkAudience := "sinkAud"
	subscriber1Audience := "subscriber1Aud"
	subscriber2Audience := "subscriber2Aud"
	filter1Audience := "filter1Aud"

	eventBody := `{"msg":"test msg"}`
	event := test.FullEvent()
	_ = event.SetData(cloudevents.ApplicationJSON, []byte(eventBody))

	cfg := []manifest.CfgFn{
		parallel.WithChannelTemplate(channelTemplate),
	}

	// Construct two branches
	branch1Num := 0
	branch2Num := 1
	subscriber1 := feature.MakeRandomK8sName("subscriber" + strconv.Itoa(branch1Num))
	subscriber2 := feature.MakeRandomK8sName("subscriber" + strconv.Itoa(branch2Num))
	filter1 := feature.MakeRandomK8sName("filter" + strconv.Itoa(branch1Num))

	f.Setup("install sink", eventshub.Install(sink,
		eventshub.OIDCReceiverAudience(sinkAudience),
		eventshub.StartReceiver))

	// Install Subscribers for both branches.
	f.Setup("install subscriber1", eventshub.Install(subscriber1,
		eventshub.ReplyWithAppendedData("appended data 1"),
		eventshub.OIDCReceiverAudience(subscriber1Audience),
		eventshub.StartReceiver))
	f.Setup("install subscriber2", eventshub.Install(subscriber2,
		eventshub.ReplyWithAppendedData("appended data 2"),
		eventshub.OIDCReceiverAudience(subscriber2Audience),
		eventshub.StartReceiver))

	// Install Filter only for first branch.
	f.Setup("install filter1", eventshub.Install(filter1,
		eventshub.ReplyWithTransformedEvent(event.Type(), event.Source(), string(event.Data())),
		eventshub.OIDCReceiverAudience(filter1Audience),
		eventshub.StartReceiver))

	// Install a Parallel with two branches
	f.Setup("install Parallel", func(ctx context.Context, t feature.T) {
		cfg = append(cfg,
			parallel.WithReply(&duckv1.Destination{
				Ref:      service.AsKReference(sink),
				Audience: &sinkAudience,
			}),
			parallel.WithSubscriberAt(branch1Num, &duckv1.Destination{
				Ref:      service.AsKReference(subscriber1),
				Audience: &subscriber1Audience,
			}),
			parallel.WithSubscriberAt(branch2Num, &duckv1.Destination{
				Ref:      service.AsKReference(subscriber2),
				Audience: &subscriber2Audience,
			}),
			parallel.WithFilterAt(branch1Num, &duckv1.Destination{
				Ref:      service.AsKReference(filter1),
				Audience: &filter1Audience,
			}),
			parallel.WithReplyAt(branch1Num, nil),
			// The Reply for second branch is same as global reply.
			parallel.WithReplyAt(branch2Num, &duckv1.Destination{
				Ref:      service.AsKReference(sink),
				Audience: &sinkAudience,
			}),
		)

		parallel.Install(parallelName, cfg...)(ctx, t)
	})
	f.Setup("Parallel goes ready", parallel.IsReady(parallelName))
	f.Setup("Parallel is addressable", parallel.IsAddressable(parallelName))

	f.Requirement("install source", eventshub.Install(
		source,
		eventshub.StartSenderToResource(parallel.GVR(), parallelName),
		eventshub.InputEvent(event),
	))

	f.Stable("test Parallel with two branches and 1 filter").
		Must("deliver event to subscriber1", eventasssert.OnStore(subscriber1).MatchEvent(test.HasId(event.ID())).AtLeast(1)).
		Must("deliver event to subscriber2", eventasssert.OnStore(subscriber2).MatchEvent(test.HasId(event.ID())).AtLeast(1)).
		Must("deliver event to filter1", eventasssert.OnStore(filter1).MatchEvent(test.HasId(event.ID())).AtLeast(1)).
		Must("deliver event from subscriber 1 to reply", eventasssert.OnStore(sink).
			MatchEvent(test.HasId(event.ID()), test.HasData([]byte("appended data 1"))).
			AtLeast(1),
		).
		Must("deliver event from subscriber 2 to reply", eventasssert.OnStore(sink).
			MatchEvent(test.HasId(event.ID()), test.HasData([]byte("appended data 2"))).
			AtLeast(1),
		)

	return f
}
