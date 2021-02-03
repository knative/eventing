package mtbroker

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1"

	. "knative.dev/pkg/configmap/testing"
)

func TestMtBrokerInternalConfig(t *testing.T) {
	_, example := ConfigMapsFromTestFile(t, "config-mtbroker-delivery")

	for _, tt := range []struct {
		name string
		fail bool
		want *eventingduck.DeliverySpec
		data *corev1.ConfigMap
	}{{
		name: "Nil config",
		fail: false,
		want: nil,
		data: nil,
	}, {
		name: "Empty config",
		fail: false,
		want: nil,
		data: &corev1.ConfigMap{},
	}, {
		name: "With values",
		fail: false,
		want: &eventingduck.DeliverySpec{Retry: pointer.Int32Ptr(10)},
		data: example,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			testConfig, err := NewInternalDeliveryConfigFromConfigMap(tt.data)
			if tt.fail != (err != nil) {
				t.Fatal("Unexpected error value:", err)
			}

			require.Equal(t, tt.want, testConfig)
		})
	}
}
