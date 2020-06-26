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
	"strings"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	. "github.com/cloudevents/sdk-go/v2/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eventingchannel "knative.dev/eventing/pkg/channel"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/eventing/test/lib/resources/sender"
)

// ChannelMessageModesAndSpecVersionsTestRunner tests the support of the channel ingress for different spec versions and message modes
func ChannelMessageModesAndSpecVersionsTestRunner(
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
				messageModeSpecVersionTest(t, channel, tc.event, tc.encoding, tc.version, options...)
			})
		}
	})
}

// Sender -> Channel -> Subscriber -> Record Events
func messageModeSpecVersionTest(t *testing.T, channel metav1.TypeMeta, event cloudevents.Event, encoding cloudevents.Encoding, version string, options ...testlib.SetupClientOption) {
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
	)

	matchers := []EventMatcher{HasExactlyAttributesEqualTo(event.Context)}
	if event.Data() != nil {
		matchers = append(matchers, HasData(event.Data()))
	} else {
		matchers = append(matchers, HasNoData())
	}
	// The extension matcher needs to match an eventual extension containing knativehistory extension
	// (which is not mandatory by the spec)
	var extKeys []string
	for k := range event.Extensions() {
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
}
