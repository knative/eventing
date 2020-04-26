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

	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/test/lib"
	"knative.dev/pkg/apis"
)

func ChannelSpecTestHelperWithChannelTestRunner(
	t *testing.T,
	channelTestRunner lib.ChannelTestRunner,
	options ...lib.SetupClientOption,
) {

	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)

		t.Run("Channel spec allows subscribers", func(t *testing.T) {
			channelSpecAllowsRequiredFields(st, client, channel)
		})
	})
}

func channelSpecAllowsRequiredFields(st *testing.T, client *lib.Client, channel metav1.TypeMeta, options ...lib.SetupClientOption) {
	st.Logf("Running channel spec conformance test with channel %q", channel)

	dtsv, err := getChannelDuckTypeSupportVersionFromTypeMeta(client, channel)
	if err != nil {
		st.Fatalf("Unable to check Channel duck type support version for %q: %q", channel, err)
	}

	channelName := names.SimpleNameGenerator.GenerateName("channel-spec-req-fields-")
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
			st.Fatalf("Unable to get channel %q to v1alpha1 duck type: %q", channel, err)
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

	} else if dtsv == "v1beta1" {
		channelable, err := getChannelAsV1Beta1Channelable(channelName, client, channel)
		if err != nil {
			st.Fatalf("Unable to get channel %q to v1beta1 duck type: %q", channel, err)
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
		st.Fatalf("Channel doesn't support v1alpha1 nor v1beta1 Channel duck types: %q", channel)
	}

	raw, err := json.Marshal(ch)
	if err != nil {
		st.Fatalf("Error marshaling %q: %q", channel, err)
	}
	u := &unstructured.Unstructured{}
	err = json.Unmarshal(raw, u)
	if err != nil {
		st.Fatalf("Error unmarshaling %q: %q", u, err)
	}

	_, err = client.Dynamic.Resource(gvr).Namespace(client.Namespace).Update(u, metav1.UpdateOptions{})
	if err != nil {
		st.Fatalf("Error updating %q with subscribers in spec: %q", channel, err)
	}
}

func getChannelDuckTypeSupportVersionFromTypeMeta(client *lib.Client, channel metav1.TypeMeta) (string, error) {
	// the only way is to create one and see
	channelName := names.SimpleNameGenerator.GenerateName("channel-tmp-")

	client.T.Logf("Creating channel %+v-%s", channel, channelName)
	client.CreateChannelOrFail(channelName, &channel)
	client.WaitForResourceReadyOrFail(channelName, &channel)

	return getChannelDuckTypeSupportVersion(channelName, client, &channel)
}
