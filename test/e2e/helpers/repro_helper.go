/*
 * Copyright 2020 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package helpers

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	testlib "knative.dev/eventing/test/lib"
)

func Repro(
	ctx context.Context,
	t *testing.T,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption) {
	const (
		channelName = "e2e-brokerchannel-channel"
	)

	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := testlib.Setup(st, true, options...)
		defer testlib.TearDown(client)

		client.CreateChannelOrFail(channelName, &channel)
		client.WaitForResourceReadyOrFail(channelName, &channel)

		// create trigger3 to receive the transformed event, and send it to the channel
		channelURL, err := client.GetAddressableURI(channelName, &channel)
		if err != nil {
			st.Fatalf("Failed to get the url for the channel %q: %+v", channelName, err)
		}
		st.Log("Found channelURL as: ", channelURL)
	})
}
