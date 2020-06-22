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
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	testCases := []struct {
		encoding cloudevents.Encoding
		version  string
	}{
		{
			encoding: cloudevents.EncodingBinary,
			version:  cloudevents.VersionV03,
		}, {
			encoding: cloudevents.EncodingStructured,
			version:  cloudevents.VersionV03,
		}, {
			encoding: cloudevents.EncodingBinary,
			version:  cloudevents.VersionV1,
		}, {
			encoding: cloudevents.EncodingStructured,
			version:  cloudevents.VersionV1,
		},
	}

	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(t *testing.T, channel metav1.TypeMeta) {
		for _, tc := range testCases {
			t.Run("encoding_"+tc.encoding.String()+"_version_"+tc.version, func(t *testing.T) {
				messageModeSpecVersionTest(t, channel, tc.encoding, tc.version, options...)
			})
		}
	})
}

// Sender -> Channel -> Subscriber -> Record Events
func messageModeSpecVersionTest(t *testing.T, channel metav1.TypeMeta, encoding cloudevents.Encoding, version string, options ...testlib.SetupClientOption) {
	client := testlib.Setup(t, true, options...)
	defer testlib.TearDown(client)

	resourcesNamePrefix := strings.ReplaceAll(strings.ToLower(encoding.String()+"-"+version), ".", "")

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

	event := FullEvent()
	event.SetID(uuid.New().String())

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

	eventTracker.AssertExact(1, recordevents.NoError(), recordevents.MatchEvent(
		HasSpecVersion(version),
		HasId(event.ID()),
		IsValid(),
	))
}
