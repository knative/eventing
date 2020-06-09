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
	"fmt"
	"testing"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"knative.dev/eventing/pkg/apis/flows/v1beta1"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing/v1beta1"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

type branchConfig struct {
	filter bool
}

func ParallelTestHelper(t *testing.T,
	channelTestRunner lib.ChannelTestRunner,
	options ...lib.SetupClientOption) {
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
	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)

		for _, tc := range table {
			parallelBranches := make([]v1beta1.ParallelBranch, len(tc.branchesConfig))
			for branchNumber, cse := range tc.branchesConfig {
				// construct filter services
				filterPodName := fmt.Sprintf("parallel-%s-branch-%d-filter", tc.name, branchNumber)
				filterPod := resources.EventFilteringPod(filterPodName, cse.filter)
				client.CreatePodOrFail(filterPod, lib.WithService(filterPodName))

				// construct branch subscriber
				subPodName := fmt.Sprintf("parallel-%s-branch-%d-sub", tc.name, branchNumber)
				subPod := resources.SequenceStepperPod(subPodName, subPodName)
				client.CreatePodOrFail(subPod, lib.WithService(subPodName))

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
			eventRecordPod := resources.EventRecordPod(eventRecorder)
			client.CreatePodOrFail(eventRecordPod, lib.WithService(eventRecorder))
			eventTracker, err := recordevents.NewEventInfoStore(client, eventRecorder)
			if err != nil {
				t.Fatalf("Pod tracker failed: %v", err)
			}
			defer eventTracker.Cleanup()

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

			client.WaitForAllTestResourcesReadyOrFail()

			// send CloudEvent to the Parallel
			event := cloudevents.NewEvent()
			event.SetID("dummy")

			eventSource := fmt.Sprintf("http://%s.svc/", senderPodName)
			event.SetSource(eventSource)
			event.SetType(lib.DefaultEventType)
			body := fmt.Sprintf(`{"msg":"TestFlowParallel %s"}`, uuid.New().String())
			if err := event.SetData(cloudevents.ApplicationJSON, []byte(body)); err != nil {
				st.Fatalf("Cannot set the payload of the event: %s", err.Error())
			}

			client.SendEventToAddressable(
				senderPodName,
				tc.name,
				lib.FlowsParallelTypeMeta,
				event)

			// verify the logger service receives the correct transformed event
			eventTracker.AssertWaitMatchSourceData(t, eventSource, tc.expected, 1, 1)
		}
	})
}
