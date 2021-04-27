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

package broker

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	logtesting "knative.dev/pkg/logging/testing"

	. "knative.dev/pkg/configmap/testing"
)

func TestOurConfig(t *testing.T) {
	actual, example := ConfigMapsFromTestFile(t, "config-broker")
	exampleSpec := runtime.RawExtension{Raw: []byte(`"customValue: foo\n"`)}

	for _, tt := range []struct {
		name    string
		wantErr string
		want    *Config
		data    *corev1.ConfigMap
	}{{
		name:    "Actual config, no defaults.",
		wantErr: "not found",
		want:    nil,
		data:    actual,
	}, {
		name: "Example config",
		want: &Config{
			DefaultChannelTemplate: messagingv1.ChannelTemplateSpec{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "messaging.knative.dev/v1",
					Kind:       "InMemoryChannel",
				},
				Spec: &exampleSpec,
			}},
		data: example,
	}, {
		name:    "Empty string for config",
		wantErr: "empty value for config",
		want:    nil,
		data: &corev1.ConfigMap{
			Data: map[string]string{
				"channelTemplateSpec": "",
			},
		},
	}, {
		name:    "Invalid json config for value",
		wantErr: `ConfigMap's value could not be unmarshaled. json: cannot unmarshal string into Go value of type v1.ChannelTemplateSpec, "asdf"`,
		want:    nil,
		data: &corev1.ConfigMap{
			Data: map[string]string{
				"channelTemplateSpec": "asdf",
			},
		},
	}, {
		name: "With values",
		want: &Config{
			DefaultChannelTemplate: messagingv1.ChannelTemplateSpec{
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
			if tt.wantErr != "" && err != nil {
				if tt.wantErr != err.Error() {
					t.Fatalf("Unexpected error value, want: %q got %q", tt.wantErr, err)
				}
			}
			if tt.wantErr == "" && err != nil {
				t.Fatalf("Unexpected error value, want no error got %q", err)
			}
			if tt.wantErr != "" && err == nil {
				t.Fatalf("Did not get wanted error, wanted %q", tt.wantErr)
			}

			t.Log(actual)

			if diff := cmp.Diff(tt.want, testConfig); diff != "" {
				if testConfig != nil && testConfig.DefaultChannelTemplate.Spec != nil {
					t.Log(string(testConfig.DefaultChannelTemplate.Spec.Raw))
				}
				t.Error("Unexpected controller config (-want, +got):", diff)
			}
		})
	}
}
