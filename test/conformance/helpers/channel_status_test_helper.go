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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/test/lib"
)

const (
	SubscribableAnnotationKey = "messaging.knative.dev/subscribable"
)

// ChannelStatusTestHelperWithChannelTestRunner runs the Channel metadata tests for all Channels in
// the ChannelTestRunner.
func ChannelStatusTestHelperWithChannelTestRunner(
	t *testing.T,
	channelTestRunner lib.ChannelTestRunner,
	options ...lib.SetupClientOption,
) {

	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)

		t.Run("Channel has required status fields", func(t *testing.T) {
			channelHasRequiredStatus(st, client, channel, options...)
		})
	})
}

func channelHasRequiredStatus(st *testing.T, client *lib.Client, channel metav1.TypeMeta, options ...lib.SetupClientOption) {
	st.Logf("Running channel status conformance test with channel %q", channel)

	channelName := "channel-req-labels"

	client.T.Logf("Creating channel %+v-%s", channel, channelName)
	client.CreateChannelOrFail(channelName, &channel)
	client.WaitForResourceReadyOrFail(channelName, &channel)

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

		// SPEC: Channel CRD MUST have a status subresource which contains address
		if channelable.Status.AddressStatus.Address == nil {
			st.Fatalf("%q does not have status.address", channel)
		}

		// SPEC: When the channel instance is ready to receive events status.address.hostname and
		// status.address.url MUST be populated
		if channelable.Status.AddressStatus.Address.Hostname == "" {
			st.Fatalf("No hostname found for %q", channel)
		}
		if channelable.Status.AddressStatus.Address.URL.IsEmpty() {
			st.Fatalf("No hostname found for %q", channel)
		}
	} else if dtsv == "v1beta1" {
		channelable, err := getChannelAsV1Beta1Channelable(channelName, client, channel)
		if err != nil {
			st.Fatalf("Unable to get channel %q to v1beta1 duck type: %q", channel, err)
		}

		// SPEC: Channel CRD MUST have a status subresource which contains address
		if channelable.Status.AddressStatus.Address == nil {
			st.Fatalf("%q does not have status.address", channel)
		}

		// SPEC: When the channel instance is ready to receive events status.address.hostname and
		// status.address.url MUST be populated
		if channelable.Status.Address.URL.IsEmpty() {
			st.Fatalf("No hostname found for %q", channel)
		}
	} else {
		st.Fatalf("Channel doesn't support v1alpha1 nor v1beta1 Channel duck types: %q", channel)
	}
}
