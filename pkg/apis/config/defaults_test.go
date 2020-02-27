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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/system"

	. "knative.dev/pkg/configmap/testing"
	_ "knative.dev/pkg/system/testing"
)

var ignoreStuff = cmp.Options{
	cmpopts.IgnoreUnexported(resource.Quantity{}),
}

func TestDefaultsConfigurationFromFile(t *testing.T) {
	_, example := ConfigMapsFromTestFile(t, DefaultsConfigName)
	if _, err := NewDefaultsConfigFromConfigMap(example); err != nil {
		t.Errorf("NewDefaultsConfigFromConfigMap(example) = %v", err)
	}
}

func TestGetBrokerConfig(t *testing.T) {
	_, example := ConfigMapsFromTestFile(t, DefaultsConfigName)
	defaults, err := NewDefaultsConfigFromConfigMap(example)
	if err != nil {
		t.Errorf("NewDefaultsConfigFromConfigMap(example) = %v", err)
	}
	c, err := defaults.GetBrokerConfig("rando")
	if err != nil {
		t.Errorf("GetBrokerConfig Failed = %v", err)
	}
	if c.Name != "somename" {
		t.Errorf("GetBrokerConfig Failed, wanted somename, got: %s", c.Name)
	}
	c, err = defaults.GetBrokerConfig("some-namespace")
	if err != nil {
		t.Errorf("GetBrokerConfig Failed = %v", err)
	}
	if c.Name != "someothername" {
		t.Errorf("GetBrokerConfig Failed, wanted someothername, got: %s", c.Name)
	}
}

func TestDefaultsConfiguration(t *testing.T) {
	configTests := []struct {
		name         string
		wantErr      bool
		wantDefaults interface{}
		config       *corev1.ConfigMap
	}{{
		name:    "defaults configuration",
		wantErr: true,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      DefaultsConfigName,
			},
			Data: map[string]string{},
		},
	}, {
		name:    "all specified values",
		wantErr: false,
		wantDefaults: &Defaults{
			NamespaceDefaultsConfig: map[string]*duckv1.KReference{
				"some-namespace": {
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Name:       "someothername",
					Namespace:  "someothernamespace",
				},
				"some-namespace-too": {
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Name:       "someothernametoo",
					Namespace:  "someothernamespacetoo",
				},
			},
			ClusterDefault: &duckv1.KReference{
				Kind:       "ConfigMap",
				APIVersion: "v1",
				Namespace:  "knative-eventing",
				Name:       "somename",
			},
		},
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      DefaultsConfigName,
			},
			Data: map[string]string{
				"default-br-config": `
      clusterDefault:
        apiVersion: v1
        kind: ConfigMap
        name: somename
        namespace: knative-eventing
      namespaceDefaults:
        some-namespace:
          apiVersion: v1
          kind: ConfigMap
          name: someothername
          namespace: someothernamespace
        some-namespace-too:
          apiVersion: v1
          kind: ConfigMap
          name: someothernametoo
          namespace: someothernamespacetoo
`,
			},
		},
	}, {
		name:    "only clusterdefault specified values",
		wantErr: false,
		wantDefaults: &Defaults{
			//			NamespaceDefaultsConfig: map[string]*duckv1.KReference{},
			ClusterDefault: &duckv1.KReference{
				Kind:       "ConfigMap",
				APIVersion: "v1",
				Namespace:  "knative-eventing",
				Name:       "somename",
			},
		},
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      DefaultsConfigName,
			},
			Data: map[string]string{
				"default-br-config": `
      clusterDefault:
        apiVersion: v1
        kind: ConfigMap
        name: somename
        namespace: knative-eventing
`,
			},
		},
	}, {
		name:    "only namespace defaults",
		wantErr: false,
		wantDefaults: &Defaults{
			NamespaceDefaultsConfig: map[string]*duckv1.KReference{
				"some-namespace": {
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Name:       "someothername",
					Namespace:  "someothernamespace",
				},
				"some-namespace-too": {
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Name:       "someothernametoo",
					Namespace:  "someothernamespacetoo",
				},
			},
		},
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      DefaultsConfigName,
			},
			Data: map[string]string{
				"default-br-config": `
      namespaceDefaults:
        some-namespace:
          apiVersion: v1
          kind: ConfigMap
          name: someothername
          namespace: someothernamespace
        some-namespace-too:
          apiVersion: v1
          kind: ConfigMap
          name: someothernametoo
          namespace: someothernamespacetoo
`,
			},
		},
	}}

	for _, tt := range configTests {
		t.Run(tt.name, func(t *testing.T) {
			actualDefaults, err := NewDefaultsConfigFromConfigMap(tt.config)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Test: %q; NewDefaultsConfigFromConfigMap() error = %v, WantErr %v", tt.name, err, tt.wantErr)
			}
			if !tt.wantErr {
				diff, err := kmp.ShortDiff(tt.wantDefaults, actualDefaults)
				if err != nil {
					t.Fatalf("Diff failed: %s %q", err, diff)
				}
				if diff != "" {
					t.Fatalf("diff %s", diff)
				}
			}
		})
	}
}
