/*
Copyright 2019 The Knative Authors

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

package defaultchannel

import (
	"testing"

	"encoding/json"

	"github.com/ghodss/yaml"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
)

var (
	config = &Config{
		ClusterDefault: &eventingduckv1alpha1.ChannelTemplateSpec{
			TypeMeta: v1.TypeMeta{
				APIVersion: messagingv1alpha1.SchemeGroupVersion.String(),
				Kind:       "InMemoryChannel",
			},
		},
	}
	// configYaml is the YAML form of config. It is generated in init().
	configYaml string

	configWithNamespace = &Config{
		ClusterDefault: &eventingduckv1alpha1.ChannelTemplateSpec{
			TypeMeta: v1.TypeMeta{
				APIVersion: messagingv1alpha1.SchemeGroupVersion.String(),
				Kind:       "InMemoryChannel",
			},
		},
		NamespaceDefaults: map[string]*eventingduckv1alpha1.ChannelTemplateSpec{
			"testNamespace": {
				TypeMeta: v1.TypeMeta{
					APIVersion: messagingv1alpha1.SchemeGroupVersion.String(),
					Kind:       "OtherChannel",
				},
			},
		},
	}
)

func init() {
	configJsonBytes, _ := json.Marshal(config)
	configYamlBytes, _ := yaml.JSONToYAML(configJsonBytes)
	configYaml = string(configYamlBytes)
}

func TestChannelDefaulter_GetDefault(t *testing.T) {
	testCases := map[string]struct {
		config                  *Config
		channel                 *messagingv1alpha1.Channel
		expectedChannelTemplate *eventingduckv1alpha1.ChannelTemplateSpec
	}{
		"no default set": {
			channel:                 &messagingv1alpha1.Channel{},
			expectedChannelTemplate: nil,
		},
		"cluster defaulted": {
			config:                  config,
			channel:                 &messagingv1alpha1.Channel{},
			expectedChannelTemplate: config.ClusterDefault,
		},
		"namespace defaulted": {
			config: configWithNamespace,
			channel: &messagingv1alpha1.Channel{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "testNamespace",
				},
			},
			expectedChannelTemplate: configWithNamespace.NamespaceDefaults["testNamespace"],
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			cd := New(zap.NewNop())
			if tc.config != nil {
				cd.setConfig(tc.config)
			}
			channelTemplate := cd.GetDefault(tc.channel.Namespace)
			if diff := cmp.Diff(tc.expectedChannelTemplate, channelTemplate); diff != "" {
				t.Fatalf("Unexpected provisioner (-want, +got): %s", diff)
			}
		})
	}
}

func TestChannelDefaulter_UpdateConfigMap(t *testing.T) {
	testCases := map[string]struct {
		initialConfig        *corev1.ConfigMap
		expectedAfterInitial *eventingduckv1alpha1.ChannelTemplateSpec
		updatedConfig        *corev1.ConfigMap
		expectedAfterUpdate  *eventingduckv1alpha1.ChannelTemplateSpec
	}{
		"nil config map": {
			expectedAfterInitial: nil,
			expectedAfterUpdate:  nil,
		},
		"key missing in update": {
			initialConfig: &corev1.ConfigMap{
				Data: map[string]string{
					channelDefaulterKey: configYaml,
				},
			},
			expectedAfterInitial: config.ClusterDefault,
			updatedConfig:        &corev1.ConfigMap{},
			expectedAfterUpdate:  config.ClusterDefault,
		},
		"bad yaml is ignored": {
			initialConfig: &corev1.ConfigMap{
				Data: map[string]string{
					channelDefaulterKey: configYaml,
				},
			},
			expectedAfterInitial: config.ClusterDefault,
			updatedConfig: &corev1.ConfigMap{
				Data: map[string]string{
					channelDefaulterKey: "foo -> bar",
				},
			},
			expectedAfterUpdate: config.ClusterDefault,
		},
		"empty config is accepted": {
			initialConfig: &corev1.ConfigMap{
				Data: map[string]string{
					channelDefaulterKey: configYaml,
				},
			},
			expectedAfterInitial: config.ClusterDefault,
			updatedConfig: &corev1.ConfigMap{
				Data: map[string]string{
					channelDefaulterKey: "{}",
				},
			},
			expectedAfterUpdate: nil,
		},
		"empty string is ignored": {
			initialConfig: &corev1.ConfigMap{
				Data: map[string]string{
					channelDefaulterKey: configYaml,
				},
			},
			expectedAfterInitial: config.ClusterDefault,
			updatedConfig: &corev1.ConfigMap{
				Data: map[string]string{
					channelDefaulterKey: "",
				},
			},
			expectedAfterUpdate: config.ClusterDefault,
		},
		"update to same channel": {
			initialConfig: &corev1.ConfigMap{
				Data: map[string]string{
					channelDefaulterKey: configYaml,
				},
			},
			expectedAfterInitial: config.ClusterDefault,
			updatedConfig: &corev1.ConfigMap{
				Data: map[string]string{
					channelDefaulterKey: configYaml,
				},
			},
			expectedAfterUpdate: config.ClusterDefault,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			cd := New(zap.NewNop())
			cd.UpdateConfigMap(tc.initialConfig)

			channelTemplate := cd.GetDefault("testNamespace")
			if diff := cmp.Diff(tc.expectedAfterInitial, channelTemplate); diff != "" {
				t.Fatalf("Unexpected difference after initial configMap update (-want, +got): %s", diff)
			}
			cd.UpdateConfigMap(tc.updatedConfig)
			channelTemplate = cd.GetDefault("testNamespace")
			if diff := cmp.Diff(tc.expectedAfterUpdate, channelTemplate); diff != "" {
				t.Fatalf("Unexpected difference after update configMap update (-want, +got): %s", diff)
			}
		})
	}
}
