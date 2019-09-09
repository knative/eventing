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

package conformance

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/uuid"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"
	"knative.dev/eventing/test/conformance/zipkin"
)

// The Channel MUST pass through all tracing information as CloudEvents attributes
func TestChannelTracing_real(t *testing.T) {
	testCases := map[string]struct {
		incomingTraceId bool
		istio           bool
	}{
		"no incoming trace id": {}, /*
			"includes incoming trace id": {
				incomingTraceId: true,
			},
			"caller has istio": {
				istio: true,
			},*/
	}

	for n, tc := range testCases {
		channelName := "chan"
		loggerPodName := "logger-pod"
		subscriptionName := "sub"
		senderName := "sendevents"
		if tc.istio {
			continue
		}
		t.Run(n, func(t *testing.T) {
			channelTestRunner.RunTests(t, common.FeatureBasic, func(st *testing.T, channel string) {
				st.Logf("Running header conformance test with channel %q", channel)
				// TODO Run in parallel.
				client := common.Setup(st, false)
				defer common.TearDown(client)

				// TODO: This probably should happen exactly once, not repeatedly.
				zipkin.SetupZipkinTracing(client.Kube.Kube, t.Logf)
				defer zipkin.CleanupZipkinTracingSetup(t.Logf)

				// create channel
				st.Logf("Creating channel")
				channelTypeMeta := common.GetChannelTypeMeta(channel)
				client.CreateChannelOrFail(channelName, channelTypeMeta)

				// create logger service as the subscriber
				pod := resources.EventDetailsPod(loggerPodName)
				client.CreatePodOrFail(pod, common.WithService(loggerPodName))

				// create subscription to subscribe the channel, and forward the received events to the logger service
				client.CreateSubscriptionOrFail(
					subscriptionName,
					channelName,
					channelTypeMeta,
					resources.WithSubscriberForSubscription(loggerPodName),
				)

				// wait for all test resources to be ready, so that we can start sending events
				if err := client.WaitForAllTestResourcesReady(); err != nil {
					st.Fatalf("Failed to get all test resources ready: %v", err)
				}

				// send fake CloudEvent to the channel
				eventID := fmt.Sprintf("%s", uuid.NewUUID())
				body := fmt.Sprintf("TestSingleHeaderEvent %s", eventID)
				event := &resources.CloudEvent{
					ID:       eventID,
					Source:   senderName,
					Type:     resources.CloudEventDefaultType,
					Data:     fmt.Sprintf(`{"msg":%q}`, body),
					Encoding: resources.CloudEventEncodingBinary,
				}

				st.Logf("Sending event with tracing headers to %s", senderName)
				if err := client.SendFakeEventWithTracingToAddressable(senderName, channelName, channelTypeMeta, event); err != nil {
					st.Fatalf("Failed to send fake CloudEvent to the channel %q", channelName)
				}

				// verify the logger service receives the event
				st.Logf("Logging for event with body %s", body)

				if err := client.CheckLog(loggerPodName, common.CheckerContains(body)); err != nil {
					st.Fatalf("String %q not found in logs of logger pod %q: %v", body, loggerPodName, err)
				}

				logs, err := client.GetLog(loggerPodName)
				if err != nil {
					st.Fatalf("Error getting logs: %v", err)
				}
				re := regexp.MustCompile("\nGot Header X-B3-Traceid: ([a-zA-Z0-9]+)\n")
				submatches := re.FindStringSubmatch(logs)
				if len(submatches) != 2 {
					st.Fatalf("Unable to extract traceID: %q", logs)
				}
				traceID := submatches[1]
				trace, err := zipkin.JSONTrace(traceID, 4, 10*time.Second)
				if err != nil {
					st.Fatalf("Unable to get trace %q: %v", traceID, err)
				}
				st.Logf("I got the trace, %q!\n%+v", traceID, trace)

				// TODO report on optional x-b3-parentspanid and x-b3-sampled if present?
				// TODO report x-custom-header

			})
		})
	}
}

// The Channel MUST pass through all tracing information as CloudEvents attributes
func TestChannelTracing(t *testing.T) {
	testCases := map[string]struct {
		incomingTraceId bool
		istio           bool
	}{
		"no incoming trace id": {}, /*
			"includes incoming trace id": {
				incomingTraceId: true,
			},
			"caller has istio": {
				istio: true,
			},*/
	}

	for n, tc := range testCases {
		channelName := "chan"
		senderName := "sendevents123"
		if tc.istio {
			continue
		}
		t.Run(n, func(t *testing.T) {
			channelTestRunner.RunTests(t, common.FeatureBasic, func(st *testing.T, channel string) {
				st.Logf("Running header conformance test with channel %q", channel)
				// TODO Run in parallel.
				client := common.Setup(st, false)

				channelTypeMeta := common.GetChannelTypeMeta(channel)

				// send fake CloudEvent to the channel
				eventID := fmt.Sprintf("%s", uuid.NewUUID())
				body := fmt.Sprintf("TestSingleHeaderEvent %s", eventID)
				event := &resources.CloudEvent{
					ID:       eventID,
					Source:   senderName,
					Type:     resources.CloudEventDefaultType,
					Data:     fmt.Sprintf(`{"msg":%q}`, body),
					Encoding: resources.CloudEventEncodingBinary,
				}

				st.Logf("Sending event with tracing headers to %s", senderName)
				if err := client.SendFakeEventWithTracingToAddressable(senderName, channelName, channelTypeMeta, event); err != nil {
					st.Fatalf("Failed to send fake CloudEvent to the channel %q", channelName)
				}
				st.Fatalf("Foo")
			})
		})
	}
}
