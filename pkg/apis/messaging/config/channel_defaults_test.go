/*
Copyright 2020 The Knative Authors.

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

package config

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/kmp"
	"knative.dev/pkg/system"

	. "knative.dev/pkg/configmap/testing"
	_ "knative.dev/pkg/system/testing"
)

func TestChannelDefaultsConfigurationFromFile(t *testing.T) {
	_, example := ConfigMapsFromTestFile(t, ChannelDefaultsConfigName)
	if _, err := NewChannelDefaultsConfigFromConfigMap(example); err != nil {
		t.Error("NewChannelDefaultsConfigFromConfigMap(example) =", err)
	}
}

func TestGetChannelConfig(t *testing.T) {
	_, example := ConfigMapsFromTestFile(t, ChannelDefaultsConfigName)
	channelDefaults, err := NewChannelDefaultsConfigFromConfigMap(example)
	if err != nil {
		t.Error("NewChannelDefaultsConfigFromConfigMap(example) =", err)
	}
	c, err := channelDefaults.GetChannelConfig("rando")
	if err != nil {
		t.Error("GetChannelConfig Failed =", err)
	}
	if c.APIVersion != "messaging.knative.dev/v1beta1" {
		t.Errorf("apiversion mismatch want %q got %q", "messaging.knative.dev/v1beta1", c.APIVersion)
	}
	if c.Kind != "InMemoryChannel" {
		t.Errorf("apiversion mismatch want %q got %q", "InMemoryChannel", c.Kind)
	}
	c, err = channelDefaults.GetChannelConfig("some-namespace")
	if err != nil {
		t.Error("GetChannelConfig Failed =", err)
	}
	if c.APIVersion != "messaging.knative.dev/v1alpha1" {
		t.Errorf("apiversion mismatch want %q got %q", "messaging.knative.dev/v1alpha1", c.APIVersion)
	}
	if c.Kind != "KafkaChannel" {
		t.Errorf("apiversion mismatch want %q got %q", "KafkaChannel", c.Kind)
	}
}

func TestChannelDefaultsConfiguration(t *testing.T) {
	configTests := []struct {
		name                string
		wantErr             bool
		wantChannelDefaults interface{}
		config              *corev1.ConfigMap
	}{{
		name:    "defaults configuration",
		wantErr: true,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      ChannelDefaultsConfigName,
			},
			Data: map[string]string{},
		},
	}, {
		name:    "all specified values",
		wantErr: false,
		wantChannelDefaults: &ChannelDefaults{
			NamespaceDefaults: map[string]*ChannelTemplateSpec{
				"some-namespace": {
					TypeMeta: v1.TypeMeta{
						APIVersion: "messaging.knative.dev/v1alpha1",
						Kind:       "KafkaChannel",
					},
				},
			},
			ClusterDefault: &ChannelTemplateSpec{
				TypeMeta: v1.TypeMeta{
					APIVersion: "messaging.knative.dev/v1beta1",
					Kind:       "InMemoryChannel",
				},
			},
		},
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      ChannelDefaultsConfigName,
			},
			Data: map[string]string{
				"default-ch-config": `
      clusterDefault:
        apiVersion: messaging.knative.dev/v1beta1
        kind: InMemoryChannel
      namespaceDefaults:
        some-namespace:
          apiVersion: messaging.knative.dev/v1alpha1
          kind: KafkaChannel
`,
			},
		},
	}, {
		name:    "only clusterdefault specified values",
		wantErr: false,
		wantChannelDefaults: &ChannelDefaults{
			ClusterDefault: &ChannelTemplateSpec{
				TypeMeta: v1.TypeMeta{
					APIVersion: "messaging.knative.dev/v1alpha1",
					Kind:       "KafkaChannel",
				},
			},
		},
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      ChannelDefaultsConfigName,
			},
			Data: map[string]string{
				"default-ch-config": `
      clusterDefault:
        apiVersion: messaging.knative.dev/v1alpha1
        kind: KafkaChannel
`,
			},
		},
	}, {
		name:    "only namespace defaults",
		wantErr: false,
		wantChannelDefaults: &ChannelDefaults{
			NamespaceDefaults: map[string]*ChannelTemplateSpec{
				"some-namespace": {
					TypeMeta: v1.TypeMeta{
						APIVersion: "messaging.knative.dev/v1beta1",
						Kind:       "InMemoryChannel",
					},
				},
			},
		},
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      ChannelDefaultsConfigName,
			},
			Data: map[string]string{
				"default-ch-config": `
      namespaceDefaults:
        some-namespace:
          apiVersion: messaging.knative.dev/v1beta1
          kind: InMemoryChannel
`,
			},
		},
	}}

	for _, tt := range configTests {
		t.Run(tt.name, func(t *testing.T) {
			actualChannelDefaults, err := NewChannelDefaultsConfigFromConfigMap(tt.config)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Test: %q; NewChannelDefaultsConfigFromConfigMap() error = %v, WantErr %v", tt.name, err, tt.wantErr)
			}
			if !tt.wantErr {
				diff, err := kmp.ShortDiff(tt.wantChannelDefaults, actualChannelDefaults)
				if err != nil {
					t.Fatalf("Diff failed: %s %q", err, diff)
				}
				if diff != "" {
					t.Fatal("diff", diff)
				}
			}
		})
	}
}
