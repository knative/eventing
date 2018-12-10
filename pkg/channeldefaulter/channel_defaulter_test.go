/*
Copyright 2018 The Knative Authors

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

package channeldefaulter

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"go.uber.org/zap"
	yaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	config = &Config{
		ClusterDefault: &corev1.ObjectReference{
			APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			Kind:       "ClusterChannelProvisioner",
			Name:       "test-channel-provisioner",
		},
	}
	// configYaml is the YAML form of config. It is generated in init().
	configYaml string

	testNamespace       = "test-namespace"
	configWithNamespace = &Config{
		ClusterDefault: &corev1.ObjectReference{
			Name: "cluster-default",
		},
		NamespaceDefaults: map[string]*corev1.ObjectReference{
			testNamespace: {
				Name: "namespace-default",
			},
		},
	}
)

func init() {
	configYamlBytes, _ := yaml.Marshal(config)
	configYaml = string(configYamlBytes)
}

func TestChannelDefaulter_getDefaultProvider(t *testing.T) {
	testCases := map[string]struct {
		nilChannelDefaulter bool
		config              *Config
		channel             *eventingv1alpha1.Channel
		expectedProv        *corev1.ObjectReference
	}{
		"nil channel defaulter": {
			nilChannelDefaulter: true,
		},
		"nil spec": {},
		"no default set": {
			channel:      &eventingv1alpha1.Channel{},
			expectedProv: nil,
		},
		"cluster defaulted": {
			config:       config,
			channel:      &eventingv1alpha1.Channel{},
			expectedProv: config.ClusterDefault,
		},
		"namespace defaulted": {
			config: configWithNamespace,
			channel: &eventingv1alpha1.Channel{
				ObjectMeta: v1.ObjectMeta{
					Namespace: testNamespace,
				},
			},
			expectedProv: configWithNamespace.NamespaceDefaults[testNamespace],
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			var cd *ChannelDefaulter
			if !tc.nilChannelDefaulter {
				cd = New(zap.NewNop())
			}
			if tc.config != nil {
				cd.setConfig(tc.config)
			}
			prov, args := cd.GetDefault(tc.channel)
			if diff := cmp.Diff(tc.expectedProv, prov); diff != "" {
				t.Fatalf("Unexpected provisioner (-want, +got): %s", diff)
			}
			if args != nil {
				t.Fatalf("Unexpected args, expected nil: %v", args)
			}
		})
	}
}

func TestChannelDefaulter_UpdateConfigMap(t *testing.T) {
	testCases := map[string]struct {
		initialConfig        *corev1.ConfigMap
		expectedAfterInitial *corev1.ObjectReference
		updatedConfig        *corev1.ConfigMap
		expectedAfterUpdate  *corev1.ObjectReference
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
					channelDefaulterKey: "{foo: bar}",
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
		"update to same provisioner": {
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
		"update to different provisioner": {
			initialConfig: &corev1.ConfigMap{
				Data: map[string]string{
					channelDefaulterKey: configYaml,
				},
			},
			expectedAfterInitial: config.ClusterDefault,
			updatedConfig: &corev1.ConfigMap{
				Data: map[string]string{
					channelDefaulterKey: strings.Replace(configYaml, config.ClusterDefault.Name, "some-other-name", -1),
				},
			},
			expectedAfterUpdate: &corev1.ObjectReference{
				APIVersion: config.ClusterDefault.APIVersion,
				Kind:       config.ClusterDefault.Kind,
				Name:       "some-other-name",
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			cd := New(zap.NewNop())
			cd.UpdateConfigMap(tc.initialConfig)

			prov, args := cd.GetDefault(&eventingv1alpha1.Channel{})
			if diff := cmp.Diff(tc.expectedAfterInitial, prov); diff != "" {
				t.Fatalf("Unexpected difference after initial configMap update (-want, +got): %s", diff)
			}
			if args != nil {
				t.Fatalf("Unexpected args after initial configMap update. Expected nil. %v", args)
			}

			cd.UpdateConfigMap(tc.updatedConfig)
			prov, args = cd.GetDefault(&eventingv1alpha1.Channel{})
			if diff := cmp.Diff(tc.expectedAfterUpdate, prov); diff != "" {
				t.Fatalf("Unexpected difference after update configMap update (-want, +got): %s", diff)
			}
			if args != nil {
				t.Fatalf("Unexpected args after initial configMap update. Expected nil. %v", args)
			}
		})
	}
}
