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
