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
	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/flows/v1alpha1"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgTest "knative.dev/pkg/test"
)

func TestFlowsParallel(t *testing.T) {
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
	channelTypeMeta := &common.DefaultChannel

	client := setup(t, true)
	defer tearDown(client)

	for _, tc := range table {
		parallelBranches := make([]v1alpha1.ParallelBranch, len(tc.branchesConfig))
		for branchNumber, cse := range tc.branchesConfig {
			// construct filter services
			filterPodName := fmt.Sprintf("parallel-%s-branch-%d-filter", tc.name, branchNumber)
			filterPod := resources.EventFilteringPod(filterPodName, cse.filter)
			client.CreatePodOrFail(filterPod, common.WithService(filterPodName))

			// construct branch subscriber
			subPodName := fmt.Sprintf("parallel-%s-branch-%d-sub", tc.name, branchNumber)
			subPod := resources.SequenceStepperPod(subPodName, subPodName)
			client.CreatePodOrFail(subPod, common.WithService(subPodName))

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
		client.CreatePodOrFail(loggerPod, common.WithService(loggerPodName))

		// create channel as reply of the Parallel
		// TODO(Fredy-Z): now we'll have to use a channel plus its subscription here, as reply of the Subscription
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

		parallel := eventingtesting.NewFlowsParallel(tc.name, client.Namespace,
			eventingtesting.WithFlowsParallelChannelTemplateSpec(channelTemplate),
			eventingtesting.WithFlowsParallelBranches(parallelBranches),
			eventingtesting.WithFlowsParallelReply(&duckv1.Destination{Ref: pkgTest.CoreV1ObjectReference(channelTypeMeta.Kind, channelTypeMeta.APIVersion, replyChannelName)}))

		client.CreateFlowsParallelOrFail(parallel)

		if err := client.WaitForAllTestResourcesReady(); err != nil {
			t.Fatalf("Failed to get all test resources ready: %v", err)
		}

		// send fake CloudEvent to the Parallel
		msg := fmt.Sprintf("TestFlowParallel %s - ", uuid.NewUUID())
		// NOTE: the eventData format must be CloudEventBaseData, as it needs to be correctly parsed in the stepper service.
		eventData := resources.CloudEventBaseData{Message: msg}
		eventDataBytes, err := json.Marshal(eventData)
		if err != nil {
			t.Fatalf("Failed to convert %v to json: %v", eventData, err)
		}
		event := &resources.CloudEvent{
			Source:   senderPodName,
			Type:     resources.CloudEventDefaultType,
			Data:     string(eventDataBytes),
			Encoding: resources.CloudEventDefaultEncoding,
		}
		if err := client.SendFakeEventToAddressable(
			senderPodName,
			tc.name,
			common.FlowsParallelTypeMeta,
			event,
		); err != nil {
			t.Fatalf("Failed to send fake CloudEvent to the parallel %q : %s", tc.name, err)
		}

		// verify the logger service receives the correct transformed event
		if err := client.CheckLog(loggerPodName, common.CheckerContains(tc.expected)); err != nil {
			t.Fatalf("String %q not found in logs of logger pod %q: %v", tc.expected, loggerPodName, err)
		}
	}

}
