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
	"fmt"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"

	duckv1 "knative.dev/pkg/apis/duck/v1"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"

	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	eventingtestingv1 "knative.dev/eventing/pkg/reconciler/testing/v1"
)

func ChannelPreferHeaderCheck(
	ctx context.Context,
	t *testing.T,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption) {
	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		const (
			eventRecord    = "event-record"
			senderName     = "sender"
			channelName    = "test-channel"
			pingSourceName = "test-ping-source-annotation"
			// Every 1 minute starting from now
			schedule = "*/1 * * * *"
		)

		tests := []struct {
			name string
		}{
			{
				name: "test messag without prefer header",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				client := testlib.Setup(t, true)
				defer testlib.TearDown(client)

				// create channel
				client.CreateChannelWithDefaultOrFail(&messagingv1.Channel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      channelName,
						Namespace: client.Namespace,
					},
					Spec: messagingv1.ChannelSpec{
						ChannelTemplate: &messagingv1.ChannelTemplateSpec{
							TypeMeta: channel,
						},
						ChannelableSpec: eventingduckv1.ChannelableSpec{
							Delivery: &eventingduckv1.DeliverySpec{
								DeadLetterSink: &duckv1.Destination{
									Ref: resources.KnativeRefForService(eventRecord, client.Namespace),
								},
								Retry: pointer.Int32Ptr(10),
							},
						},
					},
				})

				client.WaitForResourcesReadyOrFail(&channel)

				allEventTracker, _ := recordevents.StartEventRecordOrFail(
					ctx,
					client,
					eventRecord,
				)

				// wait for all test resources to be ready, so that we can start sending events
				client.WaitForAllTestResourcesReadyOrFail(ctx)

				metaResourceList := resources.NewMetaResourceList(client.Namespace, &channel)
				objs, err := duck.GetGenericObjectList(client.Dynamic, metaResourceList, &eventingduckv1.Subscribable{})
				if err != nil {
					t.Fatal("Failed to list the underlying channels:", err)
				}

				// Note that since by default MT ChannelBroker creates a Broker in each namespace, there's
				// actually two channels.
				// https://github.com/knative/eventing/issues/3138
				// So, filter out the broker channel from the list before checking that there's only one.
				filteredObjs := make([]runtime.Object, 0)
				for _, o := range objs {
					if o.(*eventingduckv1.Subscribable).Name != "default-kne-trigger" {
						filteredObjs = append(filteredObjs, o)
					}
				}

				if len(filteredObjs) != 1 {
					t.Logf("Got unexpected channels:")
					for i, ec := range filteredObjs {
						t.Logf("Extra channels: %d : %+v", i, ec)
					}
					t.Fatal("The defaultchannel is expected to create 1 underlying channel, but got", len(filteredObjs))
				}

				jsonData := fmt.Sprintf(`{"msg":"Test trigger-annotation %s"}`, uuid.NewUUID())
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
									Name:       channelName,
									APIVersion: "eventing.knative.dev/v1",
									Kind:       "Channel",
								},
							},
						},
					}),
				)

				client.CreatePingSourceV1OrFail(pingSource)

				allEventTracker.AssertAtLeast(1,
					recordevents.HasAdditionalHeader("Prefer", "reply"),
				)
			})
		}
	})
}
