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
	"net/http"
	"strconv"
	"strings"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	. "github.com/cloudevents/sdk-go/v2/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eventingchannel "knative.dev/eventing/pkg/channel"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/eventing/test/lib/sender"
)

// ChannelDataPlaneSuccessTestRunner tests the support of the channel ingress for different spec versions and message modes
func ChannelDataPlaneSuccessTestRunner(
	t *testing.T,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption,
) {
	type testCase struct {
		event    cloudevents.Event
		encoding cloudevents.Encoding
		version  string
	}

	var testCases []testCase

	// Generate matrix events/encoding/spec versions
	for _, event := range []cloudevents.Event{MinEvent(), FullEvent()} {
		for _, enc := range []cloudevents.Encoding{cloudevents.EncodingBinary, cloudevents.EncodingStructured} {
			for _, version := range []string{cloudevents.VersionV03, cloudevents.VersionV1} {
				testCases = append(testCases, testCase{ConvertEventExtensionsToString(t, event), enc, version})
			}
		}
	}

	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(t *testing.T, channel metav1.TypeMeta) {
		for _, tc := range testCases {
			t.Run(tc.event.ID()+"_encoding_"+tc.encoding.String()+"_version_"+tc.version, func(t *testing.T) {
				channelDataPlaneSuccessTest(t, channel, tc.event, tc.encoding, tc.version, options...)
			})
		}
	})
}

// Sender -> Channel -> Subscriber -> Record Events
func channelDataPlaneSuccessTest(t *testing.T, channel metav1.TypeMeta, event cloudevents.Event, encoding cloudevents.Encoding, version string, options ...testlib.SetupClientOption) {
	client := testlib.Setup(t, true, options...)
	defer testlib.TearDown(client)

	resourcesNamePrefix := strings.ReplaceAll(strings.ToLower(event.ID()+"-"+encoding.String()+"-"+version), ".", "")

	channelName := resourcesNamePrefix + "-ch"
	client.CreateChannelOrFail(channelName, &channel)

	subscriberName := resourcesNamePrefix + "-recordevents"
	eventTracker, _ := recordevents.StartEventRecordOrFail(client, subscriberName)

	client.CreateSubscriptionOrFail(
		resourcesNamePrefix+"-sub",
		channelName,
		&channel,
		resources.WithSubscriberForSubscription(subscriberName),
	)

	client.WaitForAllTestResourcesReadyOrFail()

	switch version {
	case cloudevents.VersionV1:
		event.Context = event.Context.AsV1()
	case cloudevents.VersionV03:
		event.Context = event.Context.AsV03()
	}

	client.SendEventToAddressable(
		resourcesNamePrefix+"-sender",
		channelName,
		&channel,
		event,
		sender.WithEncoding(encoding),
		sender.WithResponseSink("http://"+client.GetServiceHost(subscriberName)),
	)

	matchers := []EventMatcher{HasExactlyAttributesEqualTo(event.Context)}
	if event.Data() != nil {
		matchers = append(matchers, HasData(event.Data()))
	} else {
		matchers = append(matchers, HasNoData())
	}
	// The extension matcher needs to match an eventual extension containing knativehistory extension
	// (which is not mandatory by the spec)
	extensions := event.Extensions()
	extKeys := make([]string, 0, len(extensions))
	for k := range extensions {
		extKeys = append(extKeys, k)
	}
	extKeys = append(extKeys, eventingchannel.EventHistory)
	matchers = append(matchers, AnyOf(
		HasExactlyExtensions(event.Extensions()),
		AllOf(
			ContainsExactlyExtensions(extKeys...),
			HasExtensions(event.Extensions()),
		),
	))

	eventTracker.AssertExact(
		1,
		recordevents.NoError(),
		recordevents.MatchEvent(matchers...),
	)

	eventTracker.AssertExact(
		1,
		recordevents.MatchEvent(sender.MatchStatusCode(http.StatusAccepted)),
	)
}

// ChannelDataPlaneFailureTestRunner tests some status codes from the spec
func ChannelDataPlaneFailureTestRunner(
	t *testing.T,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption,
) {
	testCases := []struct {
		statusCode int
		senderFn   func(c *testlib.Client, senderName string, channelName string, channel metav1.TypeMeta, eventId string, responseSink string)
		eventId    string
	}{{
		statusCode: http.StatusMethodNotAllowed,
		senderFn: func(c *testlib.Client, senderName string, channelName string, channel metav1.TypeMeta, eventId string, responseSink string) {
			c.SendRequestToAddressable(
				senderName,
				channelName,
				&channel,
				map[string]string{
					"ce-specversion": "1.0",
					"ce-type":        "example",
					"ce-source":      "http://localhost",
					"ce-id":          eventId,
					"content-type":   "application/json",
				},
				"{}",
				sender.WithMethod("PUT"),
				sender.WithResponseSink(responseSink),
			)
		},
	}, {
		statusCode: http.StatusBadRequest,
		senderFn: func(c *testlib.Client, senderName string, channelName string, channel metav1.TypeMeta, eventId string, responseSink string) {
			c.SendRequestToAddressable(
				senderName,
				channelName,
				&channel,
				map[string]string{
					"ce-specversion": "10.0", // <-- Spec version not existing
					"ce-type":        "example",
					"ce-source":      "http://localhost",
					"ce-id":          eventId,
					"content-type":   "application/json",
				},
				"{}",
				sender.WithResponseSink(responseSink),
			)
		},
	}}

	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(t *testing.T, channel metav1.TypeMeta) {
		for _, tc := range testCases {
			t.Run("expecting-"+strconv.Itoa(tc.statusCode), func(t *testing.T) {
				channelDataPlaneFailureTest(t, channel, tc.senderFn, tc.statusCode, options...)
			})
		}
	})
}

// (Request) Sender -> Channel -> Subscriber -> Record Events
func channelDataPlaneFailureTest(
	t *testing.T,
	channel metav1.TypeMeta,
	senderFn func(c *testlib.Client, senderName string, channelName string, channel metav1.TypeMeta, eventId string, responseSink string),
	expectingStatusCode int,
	options ...testlib.SetupClientOption,
) {
	client := testlib.Setup(t, true, options...)
	defer testlib.TearDown(client)

	resourcesNamePrefix := fmt.Sprintf("expecting-%d", expectingStatusCode)

	channelName := resourcesNamePrefix + "-ch"
	client.CreateChannelOrFail(channelName, &channel)

	subscriberName := resourcesNamePrefix + "-recordevents"
	eventTracker, _ := recordevents.StartEventRecordOrFail(client, subscriberName)

	client.CreateSubscriptionOrFail(
		resourcesNamePrefix+"-sub",
		channelName,
		&channel,
		resources.WithSubscriberForSubscription(subscriberName),
	)

	client.WaitForAllTestResourcesReadyOrFail()

	eventId := "xyz"
	senderFn(client, resourcesNamePrefix+"-sender", channelName, channel, eventId, "http://"+client.GetServiceHost(subscriberName))

	eventTracker.AssertExact(
		1,
		recordevents.MatchEvent(sender.MatchStatusCode(expectingStatusCode)),
	)

	// Assert the event is not received
	eventTracker.AssertNot(
		recordevents.MatchEvent(HasId(eventId)),
	)
}
