/*
 * Copyright 2020 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package helpers

import (
	"context"
	"fmt"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	. "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/dropevents"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
)

// BrokerCreator creates a broker and returns its broker name.
type BrokerCreatorWithRetries func(client *testlib.Client, numRetries int32) string

func BrokerRedelivery(ctx context.Context, t *testing.T, creator BrokerCreatorWithRetries) {

	numRetries := int32(5)

	t.Run(dropevents.Fibonacci, func(t *testing.T) {
		brokerRedelivery(ctx, t, creator, numRetries, func(pod *corev1.Pod, client *testlib.Client) error {
			pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env,
				corev1.EnvVar{
					Name:  dropevents.SkipAlgorithmKey,
					Value: dropevents.Fibonacci,
				},
			)
			return nil
		})
	})

	t.Run(dropevents.Sequence, func(t *testing.T) {
		brokerRedelivery(ctx, t, creator, numRetries, func(pod *corev1.Pod, client *testlib.Client) error {
			pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env,
				corev1.EnvVar{
					Name:  dropevents.SkipAlgorithmKey,
					Value: dropevents.Sequence,
				},
				corev1.EnvVar{
					Name:  dropevents.NumberKey,
					Value: fmt.Sprintf("%d", numRetries),
				},
			)
			return nil
		})
	})
}

func brokerRedelivery(ctx context.Context, t *testing.T, creator BrokerCreatorWithRetries, numRetries int32, options ...recordevents.EventRecordOption) {

	const (
		triggerName = "trigger"
		eventRecord = "event-record"
		senderName  = "sender"

		eventType   = "type"
		eventSource = "http://source.com"
		eventBody   = `{"msg":"broker-redelivery"}`
	)

	client := testlib.Setup(t, true)
	defer testlib.TearDown(client)

	// Create event tracker that should receive all events.
	allEventTracker, _ := recordevents.StartEventRecordOrFail(
		ctx,
		client,
		eventRecord,
		options...,
	)

	// Create a Broker.
	brokerName := creator(client, numRetries)

	client.CreateTriggerV1OrFail(
		triggerName,
		resources.WithBrokerV1(brokerName),
		resources.WithSubscriberServiceRefForTriggerV1(eventRecord),
	)

	client.WaitForAllTestResourcesReadyOrFail(ctx)

	// send CloudEvent to the broker

	eventToSend := cloudevents.NewEvent()
	eventToSend.SetID(uuid.New().String())
	eventToSend.SetType(eventType)
	eventToSend.SetSource(eventSource)
	if err := eventToSend.SetData(cloudevents.ApplicationJSON, []byte(eventBody)); err != nil {
		t.Fatal("Cannot set the payload of the event:", err.Error())
	}

	client.SendEventToAddressable(ctx, senderName, brokerName, testlib.BrokerTypeMeta, eventToSend)

	allEventTracker.AssertAtLeast(1, recordevents.MatchEvent(AllOf(
		HasSource(eventSource),
		HasType(eventType),
		HasData([]byte(eventBody)),
	)))
}
