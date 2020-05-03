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
	"encoding/json"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	"knative.dev/eventing/pkg/apis/flows/v1alpha1"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/cloudevents"
	"knative.dev/eventing/test/lib/resources"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func SequenceTestHelper(t *testing.T,
	channelTestRunner lib.ChannelTestRunner,
	options ...lib.SetupClientOption) {
	const (
		sequenceName  = "e2e-sequence"
		senderPodName = "e2e-sequence-sender-pod"

		channelName      = "e2e-sequence-channel"
		subscriptionName = "e2e-sequence-subscription"
		loggerPodName    = "e2e-sequence-logger-pod"
	)

	stepSubscriberConfigs := []struct {
		podName     string
		msgAppender string
	}{{
		podName:     "e2e-stepper1",
		msgAppender: "-step1",
	}, {
		podName:     "e2e-stepper2",
		msgAppender: "-step2",
	}, {
		podName:     "e2e-stepper3",
		msgAppender: "-step3",
	}}

	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)

		// construct steps for the sequence
		steps := make([]v1alpha1.SequenceStep, 0)
		for _, config := range stepSubscriberConfigs {
			// create a stepper Pod with Service
			podName := config.podName
			msgAppender := config.msgAppender
			stepperPod := resources.SequenceStepperPod(podName, msgAppender)

			client.CreatePodOrFail(stepperPod, lib.WithService(podName))
			// create a new step
			step := v1alpha1.SequenceStep{
				Destination: duckv1.Destination{
					Ref: resources.KnativeRefForService(podName, client.Namespace),
				}}
			// add the step into steps
			steps = append(steps, step)
		}

		// create channelTemplate for the Sequence
		channelTemplate := &messagingv1beta1.ChannelTemplateSpec{
			TypeMeta: channel,
		}

		// create channel as reply of the Sequence
		// TODO(chizhg): now we'll have to use a channel plus its subscription here, as reply of the Sequence
		//                must be Addressable. In the future if we use Knative Serving in the tests, we can
		//                make the logger service as a Knative service, and remove the channel and subscription.
		client.CreateChannelOrFail(channelName, &channel)
		// create logger service as the subscriber
		loggerPod := resources.EventLoggerPod(loggerPodName)
		client.CreatePodOrFail(loggerPod, lib.WithService(loggerPodName))
		// create subscription to subscribe the channel, and forward the received events to the logger service
		client.CreateSubscriptionOrFail(
			subscriptionName,
			channelName,
			&channel,
			resources.WithSubscriberForSubscription(loggerPodName),
		)
		replyRef := &duckv1.KReference{Kind: channel.Kind, APIVersion: channel.APIVersion, Name: channelName, Namespace: client.Namespace}

		// create the sequence object
		sequence := eventingtesting.NewFlowsSequence(
			sequenceName,
			client.Namespace,
			eventingtesting.WithFlowsSequenceSteps(steps),
			eventingtesting.WithFlowsSequenceChannelTemplateSpec(channelTemplate),
			eventingtesting.WithFlowsSequenceReply(&duckv1.Destination{Ref: replyRef}),
		)

		// create Sequence or fail the test if there is an error
		client.CreateFlowsSequenceOrFail(sequence)

		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail()

		// send fake CloudEvent to the Sequence
		msg := fmt.Sprintf("TestSequence %s", uuid.NewUUID())
		// NOTE: the eventData format must be BaseData, as it needs to be correctly parsed in the stepper service.
		eventData := cloudevents.BaseData{Message: msg}
		eventDataBytes, err := json.Marshal(eventData)
		if err != nil {
			st.Fatalf("Failed to convert %v to json: %v", eventData, err)
		}
		event := cloudevents.New(
			string(eventDataBytes),
			cloudevents.WithSource(senderPodName),
		)
		client.SendFakeEventToAddressableOrFail(
			senderPodName,
			sequenceName,
			lib.FlowsSequenceTypeMeta,
			event)

		// verify the logger service receives the correct transformed event
		expectedMsg := msg
		for _, config := range stepSubscriberConfigs {
			expectedMsg += config.msgAppender
		}
		if err := client.CheckLog(loggerPodName, lib.CheckerContains(expectedMsg)); err != nil {
			st.Fatalf("String %q not found in logs of logger pod %q: %v", expectedMsg, loggerPodName, err)
		}
	})
}
