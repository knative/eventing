// +build e2e

/*
Copyright 2019 The Knative Authors
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

package e2e

import (
	"encoding/json"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/util/uuid"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgTest "knative.dev/pkg/test"

	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"

	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/cloudevents"
	"knative.dev/eventing/test/lib/resources"
)

type branchConfig struct {
	filter bool
}

func TestParallel(t *testing.T) {
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
	channelTypeMeta := &lib.DefaultChannel

	client := setup(t, true)
	defer tearDown(client)

	for _, tc := range table {
		parallelBranches := make([]v1alpha1.ParallelBranch, len(tc.branchesConfig))
		for branchNumber, cse := range tc.branchesConfig {
			// construct filter services
			filterPodName := fmt.Sprintf("parallel-%s-branch-%d-filter", tc.name, branchNumber)
			filterPod := resources.EventFilteringPod(filterPodName, cse.filter)
			client.CreatePodOrFail(filterPod, lib.WithService(filterPodName))

			// construct branch subscriber
			subPodName := fmt.Sprintf("parallel-%s-branch-%d-sub", tc.name, branchNumber)
			subPod := resources.SequenceStepperPod(subPodName, subPodName)
			client.CreatePodOrFail(subPod, lib.WithService(subPodName))

			parallelBranches[branchNumber] = v1alpha1.ParallelBranch{
				Filter: &duckv1.Destination{
					Ref: resources.ServiceRef(filterPodName),
				},
				Subscriber: duckv1.Destination{
					Ref: resources.ServiceRef(subPodName),
				},
			}
		}

		channelTemplate := &eventingduckv1alpha1.ChannelTemplateSpec{
			TypeMeta: *(channelTypeMeta),
		}

		// create logger service for global reply
		loggerPodName := fmt.Sprintf("%s-logger", tc.name)
		loggerPod := resources.EventLoggerPod(loggerPodName)
		client.CreatePodOrFail(loggerPod, lib.WithService(loggerPodName))

		// create channel as reply of the Parallel
		// TODO(chizhg): now we'll have to use a channel plus its subscription here, as reply of the Subscription
		//                must be Addressable.
		replyChannelName := fmt.Sprintf("reply-%s", tc.name)
		client.CreateChannelOrFail(replyChannelName, channelTypeMeta)
		replySubscriptionName := fmt.Sprintf("reply-%s", tc.name)
		client.CreateSubscriptionOrFail(
			replySubscriptionName,
			replyChannelName,
			channelTypeMeta,
			resources.WithSubscriberForSubscription(loggerPodName),
		)

		parallel := eventingtesting.NewParallel(tc.name, client.Namespace,
			eventingtesting.WithParallelChannelTemplateSpec(channelTemplate),
			eventingtesting.WithParallelBranches(parallelBranches),
			eventingtesting.WithParallelReply(&duckv1.Destination{Ref: pkgTest.CoreV1ObjectReference(channelTypeMeta.Kind, channelTypeMeta.APIVersion, replyChannelName)}))

		client.CreateParallelOrFail(parallel)

		if err := client.WaitForAllTestResourcesReady(); err != nil {
			t.Fatalf("Failed to get all test resources ready: %v", err)
		}

		// send fake CloudEvent to the Parallel
		msg := fmt.Sprintf("TestParallel %s - ", uuid.NewUUID())
		// NOTE: the eventData format must be BaseData, as it needs to be correctly parsed in the stepper service.
		eventData := cloudevents.BaseData{Message: msg}
		eventDataBytes, err := json.Marshal(eventData)
		if err != nil {
			t.Fatalf("Failed to convert %v to json: %v", eventData, err)
		}
		event := cloudevents.New(
			string(eventDataBytes),
			cloudevents.WithSource(senderPodName),
		)
		if err := client.SendFakeEventToAddressable(
			senderPodName,
			tc.name,
			lib.ParallelTypeMeta,
			event,
		); err != nil {
			t.Fatalf("Failed to send fake CloudEvent to the parallel %q", tc.name)
		}

		// verify the logger service receives the correct transformed event
		if err := client.CheckLog(loggerPodName, lib.CheckerContains(tc.expected)); err != nil {
			t.Fatalf("String %q not found in logs of logger pod %q: %v", tc.expected, loggerPodName, err)
		}
	}

}
