/*
Copyright 2021 The Knative Authors

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
