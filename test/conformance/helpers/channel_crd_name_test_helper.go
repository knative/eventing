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

	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testlib "knative.dev/eventing/test/lib"
)

const (
	ChannelNameSuffix = "Channel"
)

// ChannelCRDNameTestHelperWithChannelTestRunner runs the Channel CRD name tests for all
// Channel resources in the ComponentsTestRunner.
func ChannelCRDNameTestHelperWithChannelTestRunner(
	t *testing.T,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption,
) {

	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := testlib.Setup(st, true, options...)
		defer testlib.TearDown(client)

		t.Run("Channel name has required suffix", func(t *testing.T) {
			channelNameHasRequiredSuffix(st, client, channel)
		})
	})
}

func channelNameHasRequiredSuffix(st *testing.T, client *testlib.Client, channel metav1.TypeMeta) {
	// From spec: The CRD's Kind SHOULD have the suffix Channel. The name MAY be just Channel.
	if !strings.HasSuffix(channel.Kind, ChannelNameSuffix) {
		client.T.Fatalf("Kind is not suffixed with %q : %q", ChannelNameSuffix, channel)
	}
}
