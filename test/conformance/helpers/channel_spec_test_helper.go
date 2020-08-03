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

	"encoding/json"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/util/retry"

	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
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
			if channel == channelv1beta1 || channel == channelv1 {
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

		channelName := names.SimpleNameGenerator.GenerateName("channel-spec-subscribers-")
		client.T.Logf("Creating channel %+v-%s", channel, channelName)
		client.CreateChannelOrFail(channelName, &channel)
		client.WaitForResourceReadyOrFail(channelName, &channel)

		sampleUrl, _ := apis.ParseURL("http://example.com")
		gvr, _ := meta.UnsafeGuessKindToResource(channel.GroupVersionKind())

		var ch interface{}

		if dtsv == "" || dtsv == "v1alpha1" {
			// treat missing annotation value as v1alpha1, as written in the spec
			channelable, err := getChannelAsV1Alpha1Channelable(channelName, client, channel)
			if err != nil {
				st.Fatalf("Unable to get channel %s to v1alpha1 duck type: %s", channel, err)
			}

			// SPEC: each channel CRD MUST contain an array of subscribers: spec.subscribable.subscribers
			channelable.Spec.Subscribable = &eventingduckv1alpha1.Subscribable{
				Subscribers: []eventingduckv1alpha1.SubscriberSpec{
					{
						UID:      "1234",
						ReplyURI: sampleUrl,
					},
				},
			}

			ch = channelable

		} else if dtsv == "v1beta1" || dtsv == "v1" {
			channelable, err := getChannelAsV1Beta1Channelable(channelName, client, channel)
			if err != nil {
				st.Fatalf("Unable to get channel %s to v1beta1 duck type: %s", channel, err)
			}

			// SPEC: each channel CRD MUST contain an array of subscribers: spec.subscribers
			channelable.Spec.Subscribers = []eventingduckv1beta1.SubscriberSpec{
				{
					UID:      "1234",
					ReplyURI: sampleUrl,
				},
			}

			ch = channelable
		} else {
			st.Fatalf("Channel doesn't support v1alpha1 nor v1beta1 Channel duck types: %s", channel)
		}

		raw, err := json.Marshal(ch)
		if err != nil {
			st.Fatalf("Error marshaling %s: %s", channel, err)
		}
		u := &unstructured.Unstructured{}
		err = json.Unmarshal(raw, u)
		if err != nil {
			st.Fatalf("Error unmarshaling %s: %s", u, err)
		}

		_, err = client.Dynamic.Resource(gvr).Namespace(client.Namespace).Update(u, metav1.UpdateOptions{})
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
