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
	"testing"

	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	duckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChannelStatusSubscriberTestHelperWithChannelTestRunner runs the tests of
// subscriber field of status for all Channels in the ComponentsTestRunner.
func ChannelStatusSubscriberTestHelperWithChannelTestRunner(
	t *testing.T,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption,
) {

	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := testlib.Setup(st, true, options...)
		defer testlib.TearDown(client)

		t.Run("Channel has required status subscriber fields", func(t *testing.T) {
			channelHasRequiredSubscriberStatus(st, client, channel, options...)
		})
	})
}

func channelHasRequiredSubscriberStatus(st *testing.T, client *testlib.Client, channel metav1.TypeMeta, options ...testlib.SetupClientOption) {
	st.Logf("Running channel subscriber status conformance test with channel %q", channel)

	channelName := "channel-req-status-subscriber"
	subscriberServiceName := "channel-req-status-subscriber-svc"

	client.T.Logf("Creating channel %+v-%s", channel, channelName)
	client.CreateChannelOrFail(channelName, &channel)
	client.WaitForResourceReadyOrFail(channelName, &channel)

	pod := resources.EventRecordPod(subscriberServiceName + "-pod")
	client.CreatePodOrFail(pod, testlib.WithService(subscriberServiceName))

	subscription := client.CreateSubscriptionOrFail(
		subscriberServiceName,
		channelName,
		&channel,
		resources.WithSubscriberForSubscription(subscriberServiceName),
	)

	// wait for all test resources to be ready, so that we can start sending events
	client.WaitForAllTestResourcesReadyOrFail()

	dtsv, err := getChannelDuckTypeSupportVersion(channelName, client, &channel)
	if err != nil {
		st.Fatalf("Unable to check Channel duck type support version for %q: %q", channel, err)
	}

	if dtsv == "" || dtsv == "v1alpha1" {
		// treat missing annotation value as v1alpha1, as written in the spec
		channelable, err := getChannelAsV1Alpha1Channelable(channelName, client, channel)
		if err != nil {
			st.Fatalf("Unable to get channel %q to v1alpha1 duck type: %q", channel, err)
		}

		// SPEC:  Each subscription to a channel is added to the channel status.subscribableStatus.subscribers automatically.
		if channelable.Status.SubscribableStatus == nil || channelable.Status.SubscribableStatus.Subscribers == nil {
			st.Fatalf("%q does not have status.subscribers", channel)
		}
		ss := findSubscriberStatusV1Beta1(channelable.Status.SubscribableStatus.Subscribers, subscription)
		if ss == nil {
			st.Fatalf("No subscription status found for channel %q and subscription %v", channel, subscription)
		}

		// SPEC: The ready field of the subscriber identified by its uid MUST be set to True when the subscription is ready to be processed.
		if ss.Ready != corev1.ConditionTrue {
			st.Fatalf("Subscription not ready found for channel %q and subscription %v", channel, subscription)
		}
	} else if dtsv == "v1beta1" {
		channelable, err := getChannelAsV1Beta1Channelable(channelName, client, channel)
		if err != nil {
			st.Fatalf("Unable to get channel %q to v1beta1 duck type: %q", channel, err)
		}

		// SPEC: Each subscription to a channel is added to the channel status.subscribers automatically.
		if channelable.Status.Subscribers == nil {
			st.Fatalf("%q does not have status.subscribers", channel)
		}
		ss := findSubscriberStatusV1Beta1(channelable.Status.Subscribers, subscription)
		if ss == nil {
			st.Fatalf("No subscription status found for channel %q and subscription %v", channel, subscription)
		}

		// SPEC: The ready field of the subscriber identified by its uid MUST be set to True when the subscription is ready to be processed.
		if ss.Ready != corev1.ConditionTrue {
			st.Fatalf("Subscription not ready found for channel %q and subscription %v", channel, subscription)
		}
	} else if dtsv == "v1" {
		channelable, err := getChannelAsV1Channelable(channelName, client, channel)
		if err != nil {
			st.Fatalf("Unable to get channel %q to v1 duck type: %q", channel, err)
		}

		// SPEC: Each subscription to a channel is added to the channel status.subscribers automatically.
		if channelable.Status.Subscribers == nil {
			st.Fatalf("%q does not have status.subscribers", channel)
		}
		ss := findSubscriberStatusV1(channelable.Status.Subscribers, subscription)
		if ss == nil {
			st.Fatalf("No subscription status found for channel %q and subscription %v", channel, subscription)
		}

		// SPEC: The ready field of the subscriber identified by its uid MUST be set to True when the subscription is ready to be processed.
		if ss.Ready != corev1.ConditionTrue {
			st.Fatalf("Subscription not ready found for channel %q and subscription %v", channel, subscription)
		}
	} else {
		st.Fatalf("Channel doesn't support v1alpha1, v1beta1 or v1 Channel duck types: %v", channel)
	}
}

func findSubscriberStatusV1Beta1(statusArr []duckv1beta1.SubscriberStatus, subscription *eventingv1beta1.Subscription) *duckv1beta1.SubscriberStatus {
	for _, v := range statusArr {
		if v.UID == subscription.UID {
			return &v
		}
	}
	return nil
}

func findSubscriberStatusV1(statusArr []duckv1.SubscriberStatus, subscription *eventingv1beta1.Subscription) *duckv1.SubscriberStatus {
	for _, v := range statusArr {
		if v.UID == subscription.UID {
			return &v
		}
	}
	return nil
}
