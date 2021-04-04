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
	"testing"

	"encoding/json"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/util/retry"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/pkg/apis"
)

func ChannelSpecTestHelperWithChannelTestRunner(
	t *testing.T,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption,
) {

	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := testlib.Setup(st, true, options...)
		defer testlib.TearDown(client)

		t.Run("Channel spec allows subscribers", func(t *testing.T) {
			if channel == channelv1 {
				t.Skip("Not running spec.subscribers array test for generic Channel")
			}
			channelSpecAllowsSubscribersArray(st, client, channel)
		})
	})
}

func channelSpecAllowsSubscribersArray(st *testing.T, client *testlib.Client, channel metav1.TypeMeta, options ...testlib.SetupClientOption) {
	st.Logf("Running channel spec conformance test with channel %s", channel)

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		dtsv, err := getChannelDuckTypeSupportVersionFromTypeMeta(client, channel)
		if err != nil {
			st.Fatalf("Unable to check Channel duck type support version for %s: %s", channel, err)
		}
		if dtsv != "v1" {
			st.Fatalf("Unexpected duck type version, wanted [v1] got: %s", dtsv)
		}
		channelName := names.SimpleNameGenerator.GenerateName("channel-spec-subscribers-")
		client.T.Logf("Creating channel %+v-%s", channel, channelName)
		client.CreateChannelOrFail(channelName, &channel)
		client.WaitForResourceReadyOrFail(channelName, &channel)

		sampleUrl := apis.HTTP("example.com")
		gvr, _ := meta.UnsafeGuessKindToResource(channel.GroupVersionKind())

		var ch interface{}

		channelable, err := getChannelAsChannelable(channelName, client, channel)
		if err != nil {
			st.Fatalf("Unable to get channel %s to v1 duck type: %s", channel, err)
		}

		// SPEC: each channel CRD MUST contain an array of subscribers: spec.subscribers
		channelable.Spec.Subscribers = []eventingduckv1.SubscriberSpec{
			{
				UID:      "1234",
				ReplyURI: sampleUrl,
			},
		}

		ch = channelable

		raw, err := json.Marshal(ch)
		if err != nil {
			st.Fatalf("Error marshaling %s: %s", channel, err)
		}
		u := &unstructured.Unstructured{}
		err = json.Unmarshal(raw, u)
		if err != nil {
			st.Fatalf("Error unmarshaling %s: %s", u, err)
		}

		err = client.RetryWebhookErrors(func(attempt int) error {
			_, e := client.Dynamic.Resource(gvr).Namespace(client.Namespace).Update(context.Background(), u, metav1.UpdateOptions{})
			if e != nil {
				client.T.Logf("Failed to update channel spec at attempt %d %q %q: %v", attempt, channel.Kind, channelName, e)
			}
			return e
		})
		return err
	})
	if err != nil {
		st.Fatalf("Error updating %s with subscribers in spec: %s", channel, err)
	}
}

func getChannelDuckTypeSupportVersionFromTypeMeta(client *testlib.Client, channel metav1.TypeMeta) (string, error) {
	// the only way is to create one and see
	channelName := names.SimpleNameGenerator.GenerateName("channel-tmp-")

	client.T.Logf("Creating channel %+v-%s", channel, channelName)
	client.CreateChannelOrFail(channelName, &channel)
	client.WaitForResourceReadyOrFail(channelName, &channel)

	return getChannelDuckTypeSupportVersion(channelName, client, &channel)
}
