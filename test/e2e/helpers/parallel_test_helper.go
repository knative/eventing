/*
Copyright 2020 The Knative Authors
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

package helpers

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	flowsv1 "knative.dev/eventing/pkg/apis/flows/v1"
	"knative.dev/eventing/pkg/apis/flows/v1beta1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	eventingtestingv1 "knative.dev/eventing/pkg/reconciler/testing/v1"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing/v1beta1"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
)

type branchConfig struct {
	filter bool
}

func ParallelTestHelper(
	ctx context.Context,
	t *testing.T,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption) {
	const (
		senderPodName = "e2e-parallel"
	)
	table := []struct {
		name           string
		branchesConfig []branchConfig
		expected       string
	}{
		{
			name: "two-branches-pass-first-branch-only",
			branchesConfig: []branchConfig{
				{filter: false},
				{filter: true},
			},
			expected: "parallel-two-branches-pass-first-branch-only-branch-0-sub",
		},
	}
	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := testlib.Setup(st, true, options...)
		defer testlib.TearDown(client)

		for _, tc := range table {
			parallelBranches := make([]v1beta1.ParallelBranch, len(tc.branchesConfig))
			for branchNumber, cse := range tc.branchesConfig {
				// construct filter services
				filterPodName := fmt.Sprintf("parallel-%s-branch-%d-filter", tc.name, branchNumber)
				if cse.filter {
					recordevents.DeployEventRecordOrFail(ctx, client, filterPodName)
				} else {
					recordevents.DeployEventRecordOrFail(ctx, client, filterPodName, recordevents.EchoEvent)
				}

				// construct branch subscriber
				subPodName := fmt.Sprintf("parallel-%s-branch-%d-sub", tc.name, branchNumber)
				recordevents.DeployEventRecordOrFail(
					ctx,
					client,
					subPodName,
					recordevents.ReplyWithAppendedData(subPodName),
				)

				parallelBranches[branchNumber] = v1beta1.ParallelBranch{
					Filter: &duckv1.Destination{
						Ref: resources.KnativeRefForService(filterPodName, client.Namespace),
					},
					Subscriber: duckv1.Destination{
						Ref: resources.KnativeRefForService(subPodName, client.Namespace),
					},
				}
			}

			channelTemplate := &messagingv1beta1.ChannelTemplateSpec{
				TypeMeta: channel,
			}

			// create event logger pod and service
			eventRecorder := fmt.Sprintf("%s-event-record-pod", tc.name)
			eventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, eventRecorder)

			// create channel as reply of the Parallel
			// TODO(chizhg): now we'll have to use a channel plus its subscription here, as reply of the Subscription
			//                must be Addressable.
			replyChannelName := fmt.Sprintf("reply-%s", tc.name)
			client.CreateChannelOrFail(replyChannelName, &channel)
			replySubscriptionName := fmt.Sprintf("reply-%s", tc.name)
			client.CreateSubscriptionOrFail(
				replySubscriptionName,
				replyChannelName,
				&channel,
				resources.WithSubscriberForSubscription(eventRecorder),
			)

			parallel := eventingtesting.NewFlowsParallel(tc.name, client.Namespace,
				eventingtesting.WithFlowsParallelChannelTemplateSpec(channelTemplate),
				eventingtesting.WithFlowsParallelBranches(parallelBranches),
				eventingtesting.WithFlowsParallelReply(&duckv1.Destination{Ref: &duckv1.KReference{Kind: channel.Kind, APIVersion: channel.APIVersion, Name: replyChannelName, Namespace: client.Namespace}}))

			client.CreateFlowsParallelOrFail(parallel)

			client.WaitForAllTestResourcesReadyOrFail(ctx)

			// send CloudEvent to the Parallel
			event := cloudevents.NewEvent()
			event.SetID("dummy")

			eventSource := fmt.Sprintf("http://%s.svc/", senderPodName)
			event.SetSource(eventSource)
			event.SetType(testlib.DefaultEventType)
			event.SetTime(time.Now())
			body := fmt.Sprintf(`{"msg":"TestFlowParallel %s"}`, uuid.New().String())
			if err := event.SetData(cloudevents.ApplicationJSON, []byte(body)); err != nil {
				st.Fatalf("Cannot set the payload of the event: %s", err.Error())
			}

			client.SendEventToAddressable(
				ctx,
				senderPodName,
				tc.name,
				testlib.FlowsParallelTypeMeta,
				event,
			)

			// verify the logger service receives the correct transformed event
			eventTracker.AssertExact(1, recordevents.MatchEvent(
				HasSource(eventSource),
				DataContains(tc.expected),
			))
		}
	})
}

func ParallelV1TestHelper(
	ctx context.Context,
	t *testing.T,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption) {
	const (
		senderPodName = "e2e-parallel"
	)
	table := []struct {
		name           string
		branchesConfig []branchConfig
		expected       string
	}{
		{
			name: "two-branches-pass-first-branch-only",
			branchesConfig: []branchConfig{
				{filter: false},
				{filter: true},
			},
			expected: "parallel-two-branches-pass-first-branch-only-branch-0-sub",
		},
	}
	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := testlib.Setup(st, true, options...)
		defer testlib.TearDown(client)

		for _, tc := range table {
			parallelBranches := make([]flowsv1.ParallelBranch, len(tc.branchesConfig))
			for branchNumber, cse := range tc.branchesConfig {
				// construct filter services
				filterPodName := fmt.Sprintf("parallel-%s-branch-%d-filter", tc.name, branchNumber)
				if cse.filter {
					recordevents.DeployEventRecordOrFail(ctx, client, filterPodName)
				} else {
					recordevents.DeployEventRecordOrFail(ctx, client, filterPodName, recordevents.EchoEvent)
				}

				// construct branch subscriber
				subPodName := fmt.Sprintf("parallel-%s-branch-%d-sub", tc.name, branchNumber)
				recordevents.DeployEventRecordOrFail(
					ctx,
					client,
					subPodName,
					recordevents.ReplyWithAppendedData(subPodName),
				)

				parallelBranches[branchNumber] = flowsv1.ParallelBranch{
					Filter: &duckv1.Destination{
						Ref: resources.KnativeRefForService(filterPodName, client.Namespace),
					},
					Subscriber: duckv1.Destination{
						Ref: resources.KnativeRefForService(subPodName, client.Namespace),
					},
				}
			}

			channelTemplate := &messagingv1.ChannelTemplateSpec{
				TypeMeta: channel,
			}

			// create event logger pod and service
			eventRecorder := fmt.Sprintf("%s-event-record-pod", tc.name)
			eventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, eventRecorder)

			// create channel as reply of the Parallel
			// TODO(chizhg): now we'll have to use a channel plus its subscription here, as reply of the Subscription
			//                must be Addressable.
			replyChannelName := fmt.Sprintf("reply-%s", tc.name)
			client.CreateChannelOrFail(replyChannelName, &channel)
			replySubscriptionName := fmt.Sprintf("reply-%s", tc.name)
			client.CreateSubscriptionOrFail(
				replySubscriptionName,
				replyChannelName,
				&channel,
				resources.WithSubscriberForSubscription(eventRecorder),
			)

			parallel := eventingtestingv1.NewFlowsParallel(tc.name, client.Namespace,
				eventingtestingv1.WithFlowsParallelChannelTemplateSpec(channelTemplate),
				eventingtestingv1.WithFlowsParallelBranches(parallelBranches),
				eventingtestingv1.WithFlowsParallelReply(&duckv1.Destination{Ref: &duckv1.KReference{Kind: channel.Kind, APIVersion: channel.APIVersion, Name: replyChannelName, Namespace: client.Namespace}}))

			client.CreateFlowsParallelV1OrFail(parallel)

			client.WaitForAllTestResourcesReadyOrFail(ctx)

			// send CloudEvent to the Parallel
			event := cloudevents.NewEvent()
			event.SetID("dummy")

			eventSource := fmt.Sprintf("http://%s.svc/", senderPodName)
			event.SetSource(eventSource)
			event.SetType(testlib.DefaultEventType)
			event.SetTime(time.Now())
			body := fmt.Sprintf(`{"msg":"TestFlowParallel %s"}`, uuid.New().String())
			if err := event.SetData(cloudevents.ApplicationJSON, []byte(body)); err != nil {
				st.Fatalf("Cannot set the payload of the event: %s", err.Error())
			}

			client.SendEventToAddressable(
				ctx,
				senderPodName,
				tc.name,
				testlib.FlowsParallelTypeMeta,
				event,
			)

			// verify the logger service receives the correct transformed event
			eventTracker.AssertExact(1, recordevents.MatchEvent(
				HasSource(eventSource),
				DataContains(tc.expected),
			))
		}
	})
}
