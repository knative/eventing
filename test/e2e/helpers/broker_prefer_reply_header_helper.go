/*
 * Copyright 2021 The Knative Authors
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
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"

	duckv1 "knative.dev/pkg/apis/duck/v1"

	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	eventingtestingv1 "knative.dev/eventing/pkg/reconciler/testing/v1"
)

func BrokerPreferHeaderCheck(
	ctx context.Context,
	brokerClass string,
	t *testing.T,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption) {
	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		const (
			eventRecord          = "event-record"
			triggerName          = "test-trigger"
			dependencyAnnotation = `{"kind":"PingSource","name":"test-ping-source-annotation","apiVersion":"sources.knative.dev/v1beta2"}`
			pingSourceName       = "test-ping-source-annotation"
			// Every 1 minute starting from now
			schedule = "*/1 * * * *"
		)

		tests := []struct {
			name string
		}{
			{
				name: "test messag without explicit prefer header should have the header",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				client := testlib.Setup(t, true)
				defer testlib.TearDown(client)

				brokerName := ChannelBasedBrokerCreator(channel, brokerClass)(client, "v1")

				// Create event tracker that should receive all events.
				allEventTracker, _ := recordevents.StartEventRecordOrFail(
					ctx,
					client,
					eventRecord,
				)

				client.WaitForAllTestResourcesReadyOrFail(ctx)

				client.CreateTriggerOrFail(triggerName,
					resources.WithSubscriberServiceRefForTrigger(eventRecord),
					resources.WithDependencyAnnotationTrigger(dependencyAnnotation),
					resources.WithBroker(brokerName),
				)

				jsonData := `{"msg":"Test msg"}`
				pingSource := eventingtestingv1.NewPingSource(
					pingSourceName,
					client.Namespace,
					eventingtestingv1.WithPingSourceSpec(sourcesv1.PingSourceSpec{
						Schedule:    schedule,
						ContentType: cloudevents.ApplicationJSON,
						Data:        jsonData,
						SourceSpec: duckv1.SourceSpec{
							Sink: duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       brokerName,
									APIVersion: "eventing.knative.dev/v1",
									Kind:       "Broker",
								},
							},
						},
					}),
				)

				client.CreatePingSourceV1OrFail(pingSource)

				// Trigger should become ready after pingSource was created
				client.WaitForResourceReadyOrFail(triggerName, testlib.TriggerTypeMeta)

				allEventTracker.AssertExact(
					1,
					recordevents.HasAdditionalHeader("Prefer", "reply"),
				)
			})
		}
	})
}
