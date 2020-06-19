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

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

func SequenceTestHelper(
	ctx context.Context,
	t *testing.T,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption) {
	const (
		sequenceName  = "e2e-sequence"
		senderPodName = "e2e-sequence-sender-pod"

		channelName         = "e2e-sequence-channel"
		subscriptionName    = "e2e-sequence-subscription"
		recordEventsPodName = "e2e-sequence-recordevents-pod"
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

	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := testlib.Setup(st, true, options...)
		defer testlib.TearDown(client)

		// construct steps for the sequence
		steps := make([]v1beta1.SequenceStep, 0)
		for _, config := range stepSubscriberConfigs {
			// create a stepper Pod with Service
			podName := config.podName
			msgAppender := config.msgAppender
			stepperPod := resources.SequenceStepperPod(podName, msgAppender)

			client.CreatePodOrFail(stepperPod, testlib.WithService(podName))
			// create a new step
			step := v1beta1.SequenceStep{
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
		// create event logger pod and service as the subscriber
		eventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, recordEventsPodName)
		// create subscription to subscribe the channel, and forward the received events to the logger service
		client.CreateSubscriptionOrFail(
			subscriptionName,
			channelName,
			&channel,
			resources.WithSubscriberForSubscription(recordEventsPodName),
		)
		replyRef := &duckv1.KReference{Kind: channel.Kind, APIVersion: channel.APIVersion, Name: channelName, Namespace: client.Namespace}

		// create the sequence object
		sequence := eventingtesting.NewSequence(
			sequenceName,
			client.Namespace,
			eventingtesting.WithSequenceSteps(steps),
			eventingtesting.WithSequenceChannelTemplateSpec(channelTemplate),
			eventingtesting.WithSequenceReply(&duckv1.Destination{Ref: replyRef}),
		)

		// create Sequence or fail the test if there is an error
		client.CreateFlowsSequenceOrFail(sequence)

		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail(ctx)

		// send CloudEvent to the Sequence
		event := cloudevents.NewEvent()
		event.SetID("dummy")
		eventSource := fmt.Sprintf("http://%s.svc/", senderPodName)
		event.SetSource(eventSource)
		event.SetType(testlib.DefaultEventType)
		msg := fmt.Sprintf("TestSequence %s", uuid.New().String())
		body := fmt.Sprintf(`{"msg":"%s"}`, msg)
		if err := event.SetData(cloudevents.ApplicationJSON, []byte(body)); err != nil {
			st.Fatalf("Cannot set the payload of the event: %s", err.Error())
		}
		client.SendEventToAddressable(
			ctx,
			senderPodName,
			sequenceName,
			testlib.FlowsSequenceTypeMeta,
			event)

		// verify the logger service receives the correct transformed event
		expectedMsg := msg
		for _, config := range stepSubscriberConfigs {
			expectedMsg += config.msgAppender
		}
		eventTracker.AssertAtLeast(1, recordevents.MatchEvent(
			cetest.HasSource(eventSource),
			cetest.DataContains(expectedMsg),
		))
	})
}

func SequenceV1TestHelper(
	ctx context.Context,
	t *testing.T,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption) {
	const (
		sequenceName  = "e2e-sequence"
		senderPodName = "e2e-sequence-sender-pod"

		channelName         = "e2e-sequence-channel"
		subscriptionName    = "e2e-sequence-subscription"
		recordEventsPodName = "e2e-sequence-recordevents-pod"
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

	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := testlib.Setup(st, true, options...)
		defer testlib.TearDown(client)

		// construct steps for the sequence
		steps := make([]flowsv1.SequenceStep, 0)
		for _, config := range stepSubscriberConfigs {
			// create a stepper Pod with Service
			podName := config.podName
			msgAppender := config.msgAppender
			stepperPod := resources.SequenceStepperPod(podName, msgAppender)

			client.CreatePodOrFail(stepperPod, testlib.WithService(podName))
			// create a new step
			step := flowsv1.SequenceStep{
				Destination: duckv1.Destination{
					Ref: resources.KnativeRefForService(podName, client.Namespace),
				}}
			// add the step into steps
			steps = append(steps, step)
		}

		// create channelTemplate for the Sequence
		channelTemplate := &messagingv1.ChannelTemplateSpec{
			TypeMeta: channel,
		}

		// create channel as reply of the Sequence
		// TODO(chizhg): now we'll have to use a channel plus its subscription here, as reply of the Sequence
		//                must be Addressable. In the future if we use Knative Serving in the tests, we can
		//                make the logger service as a Knative service, and remove the channel and subscription.
		client.CreateChannelOrFail(channelName, &channel)
		// create event logger pod and service as the subscriber
		eventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, recordEventsPodName)
		// create subscription to subscribe the channel, and forward the received events to the logger service
		client.CreateSubscriptionOrFail(
			subscriptionName,
			channelName,
			&channel,
			resources.WithSubscriberForSubscription(recordEventsPodName),
		)
		replyRef := &duckv1.KReference{Kind: channel.Kind, APIVersion: channel.APIVersion, Name: channelName, Namespace: client.Namespace}

		// create the sequence object
		sequence := eventingtestingv1.NewSequence(
			sequenceName,
			client.Namespace,
			eventingtestingv1.WithSequenceSteps(steps),
			eventingtestingv1.WithSequenceChannelTemplateSpec(channelTemplate),
			eventingtestingv1.WithSequenceReply(&duckv1.Destination{Ref: replyRef}),
		)

		// create Sequence or fail the test if there is an error
		client.CreateFlowsSequenceV1OrFail(sequence)

		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail(ctx)

		// send CloudEvent to the Sequence
		event := cloudevents.NewEvent()
		event.SetID("dummy")
		eventSource := fmt.Sprintf("http://%s.svc/", senderPodName)
		event.SetSource(eventSource)
		event.SetType(testlib.DefaultEventType)
		msg := fmt.Sprintf("TestSequence %s", uuid.New().String())
		body := fmt.Sprintf(`{"msg":"%s"}`, msg)
		if err := event.SetData(cloudevents.ApplicationJSON, []byte(body)); err != nil {
			st.Fatalf("Cannot set the payload of the event: %s", err.Error())
		}
		client.SendEventToAddressable(
			ctx,
			senderPodName,
			sequenceName,
			testlib.FlowsSequenceTypeMeta,
			event)

		// verify the logger service receives the correct transformed event
		expectedMsg := msg
		for _, config := range stepSubscriberConfigs {
			expectedMsg += config.msgAppender
		}
		eventTracker.AssertAtLeast(1, recordevents.MatchEvent(
			cetest.HasSource(eventSource),
			cetest.DataContains(expectedMsg),
		))
	})
}
