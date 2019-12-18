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
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgTest "knative.dev/pkg/test"
)

func TestFlowsSequence(t *testing.T) {
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
	channelTypeMeta := &common.DefaultChannel

	client := setup(t, true)
	defer tearDown(client)

	// construct steps for the sequence
	steps := make([]duckv1.Destination, 0)
	for _, config := range stepSubscriberConfigs {
		// create a stepper Pod with Service
		podName := config.podName
		msgAppender := config.msgAppender
		stepperPod := resources.SequenceStepperPod(podName, msgAppender)

		client.CreatePodOrFail(stepperPod, common.WithService(podName))
		// create a new step
		step := duckv1.Destination{
			Ref: resources.ServiceRef(podName),
		}
		// add the step into steps
		steps = append(steps, step)
	}

	// create channelTemplate for the Sequence
	channelTemplate := &eventingduckv1alpha1.ChannelTemplateSpec{
		TypeMeta: *(channelTypeMeta),
	}

	// create channel as reply of the Sequence
	// TODO(Fredy-Z): now we'll have to use a channel plus its subscription here, as reply of the Sequence
	//                must be Addressable. In the future if we use Knative Serving in the tests, we can
	//                make the logger service as a Knative service, and remove the channel and subscription.
	client.CreateChannelOrFail(channelName, channelTypeMeta)
	// create logger service as the subscriber
	loggerPod := resources.EventLoggerPod(loggerPodName)
	client.CreatePodOrFail(loggerPod, common.WithService(loggerPodName))
	// create subscription to subscribe the channel, and forward the received events to the logger service
	client.CreateSubscriptionOrFail(
		subscriptionName,
		channelName,
		channelTypeMeta,
		resources.WithSubscriberForSubscription(loggerPodName),
	)
	replyRef := pkgTest.CoreV1ObjectReference(channelTypeMeta.Kind, channelTypeMeta.APIVersion, channelName)

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
	if err := client.WaitForAllTestResourcesReady(); err != nil {
		t.Fatalf("Failed to get all test resources ready: %v", err)
	}

	// send fake CloudEvent to the Sequence
	msg := fmt.Sprintf("TestSequence %s", uuid.NewUUID())
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
		sequenceName,
		common.FlowsSequenceTypeMeta,
		event,
	); err != nil {
		t.Fatalf("Failed to send fake CloudEvent to the sequence %q : %s", sequenceName, err)
	}

	// verify the logger service receives the correct transformed event
	expectedMsg := msg
	for _, config := range stepSubscriberConfigs {
		expectedMsg += config.msgAppender
	}
	if err := client.CheckLog(loggerPodName, common.CheckerContains(expectedMsg)); err != nil {
		t.Fatalf("String %q not found in logs of logger pod %q: %v", expectedMsg, loggerPodName, err)
	}
}
