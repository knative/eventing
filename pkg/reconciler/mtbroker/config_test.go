/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mtbroker

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	logtesting "knative.dev/pkg/logging/testing"

	. "knative.dev/pkg/configmap/testing"
)

func TestOurConfig(t *testing.T) {
	actual, example := ConfigMapsFromTestFile(t, "config-broker")
	exampleSpec := runtime.RawExtension{Raw: []byte(`"customValue: foo\n"`)}

	for _, tt := range []struct {
		name string
		fail bool
		want *Config
		data *corev1.ConfigMap
	}{{
		name: "Actual config, no defaults.",
		fail: true,
		want: nil,
		data: actual,
	}, {
		name: "Example config",
		fail: false,
		want: &Config{
			DefaultChannelTemplate: messagingv1beta1.ChannelTemplateSpec{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "messaging.knative.dev/v1alpha1",
					Kind:       "InMemoryChannel",
				},
				Spec: &exampleSpec,
			}},
		data: example,
	}, {
		name: "With values",
		want: &Config{
			DefaultChannelTemplate: messagingv1beta1.ChannelTemplateSpec{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "Foo/v1",
					Kind:       "Bar",
				},
			},
		},
		data: &corev1.ConfigMap{
			Data: map[string]string{
				"channelTemplateSpec": `
      apiVersion: Foo/v1
      kind: Bar
`,
			},
		},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			testConfig, err := NewConfigFromConfigMapFunc(logtesting.TestContextWithLogger(t))(tt.data)
			if tt.fail != (err != nil) {
				t.Fatalf("Unexpected error value: %v", err)
			}

			t.Log(actual)

			if diff := cmp.Diff(tt.want, testConfig); diff != "" {
				if testConfig != nil && testConfig.DefaultChannelTemplate.Spec != nil {
					t.Log(string(testConfig.DefaultChannelTemplate.Spec.Raw))
				}
				t.Errorf("Unexpected controller config (-want, +got): %s", diff)
			}
		})
	}
}
