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
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/system"

	. "knative.dev/pkg/configmap/testing"
	_ "knative.dev/pkg/system/testing"
)

func TestDefaultsConfigurationFromFile(t *testing.T) {
	_, example := ConfigMapsFromTestFile(t, DefaultsConfigName)
	if _, err := NewDefaultsConfigFromConfigMap(example); err != nil {
		t.Error("NewDefaultsConfigFromConfigMap(example) =", err)
	}
}

func TestGetBrokerConfig(t *testing.T) {
	_, example := ConfigMapsFromTestFile(t, DefaultsConfigName)
	defaults, err := NewDefaultsConfigFromConfigMap(example)
	if err != nil {
		t.Error("NewDefaultsConfigFromConfigMap(example) =", err)
	}
	c, err := defaults.GetBrokerConfig("rando")
	if err != nil {
		t.Error("GetBrokerConfig Failed =", err)
	}
	if c.Name != "somename" {
		t.Error("GetBrokerConfig Failed, wanted somename, got:", c.Name)
	}
	c, err = defaults.GetBrokerConfig("some-namespace")
	if err != nil {
		t.Error("GetBrokerConfig Failed =", err)
	}
	if c.Name != "someothername" {
		t.Error("GetBrokerConfig Failed, wanted someothername, got:", c.Name)
	}

	// Nil and empty tests
	var nilDefaults *Defaults
	_, err = nilDefaults.GetBrokerConfig("rando")
	if err == nil {
		t.Errorf("GetBrokerConfig did not fail with nil")
	}
	if err.Error() != "Defaults are nil" {
		t.Error("GetBrokerConfig did not fail with nil msg, got", err)
	}
	emptyDefaults := Defaults{}
	_, err = emptyDefaults.GetBrokerConfig("rando")
	if err == nil {
		t.Errorf("GetBrokerConfig did not fail with empty")
	}
	if err.Error() != "Defaults for Broker Configurations have not been set up." {
		t.Error("GetBrokerConfig did not fail with non-setup msg, got", err)
	}
}

func TestGetBrokerClass(t *testing.T) {
	_, example := ConfigMapsFromTestFile(t, DefaultsConfigName)
	defaults, err := NewDefaultsConfigFromConfigMap(example)
	if err != nil {
		t.Error("NewDefaultsConfigFromConfigMap(example) =", err)
	}
	c, err := defaults.GetBrokerClass("rando")
	if err != nil {
		t.Error("GetBrokerClass Failed =", err)
	}
	if c != "MTChannelBasedBroker" {
		t.Error("GetBrokerClass Failed, wanted MTChannelBasedBroker, got:", c)
	}
	c, err = defaults.GetBrokerClass("some-namespace")
	if err != nil {
		t.Error("GetBrokerClass Failed =", err)
	}
	if c != "someotherbrokerclass" {
		t.Error("GetBrokerClass Failed, wanted someothername, got:", c)
	}

	// Nil and empty tests
	var nilDefaults *Defaults
	_, err = nilDefaults.GetBrokerClass("rando")
	if err == nil {
		t.Errorf("GetBrokerClass did not fail with nil")
	}
	if err.Error() != "Defaults are nil" {
		t.Error("GetBrokerClass did not fail with nil msg, got", err)
	}
	emptyDefaults := Defaults{}
	_, err = emptyDefaults.GetBrokerClass("rando")
	if err == nil {
		t.Errorf("GetBrokerClass did not fail with empty")
	}
	if err.Error() != "Defaults for Broker Configurations have not been set up." {
		t.Error("GetBrokerClass did not fail with non-setup msg, got", err)
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
			NamespaceDefaultsConfig: map[string]*ClassAndBrokerConfig{
				"some-namespace": {
					BrokerClass: "somenamespaceclass",
					BrokerConfig: &BrokerConfig{
						KReference: &duckv1.KReference{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       "someothername",
							Namespace:  "someothernamespace",
						},
						Delivery: nil,
					},
				},
				"some-namespace-too": {
					BrokerClass: "somenamespaceclasstoo",
					BrokerConfig: &BrokerConfig{
						KReference: &duckv1.KReference{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       "someothernametoo",
							Namespace:  "someothernamespacetoo",
						},
						Delivery: nil,
					},
				},
			},
			ClusterDefault: &ClassAndBrokerConfig{
				BrokerClass: "clusterbrokerclass",
				BrokerConfig: &BrokerConfig{
					KReference: &duckv1.KReference{
						Kind:       "ConfigMap",
						APIVersion: "v1",
						Namespace:  "knative-eventing",
						Name:       "somename",
					},
					Delivery: nil,
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
      clusterDefault:
        brokerClass: clusterbrokerclass
        apiVersion: v1
        kind: ConfigMap
        name: somename
        namespace: knative-eventing
      namespaceDefaults:
        some-namespace:
          brokerClass: somenamespaceclass
          apiVersion: v1
          kind: ConfigMap
          name: someothername
          namespace: someothernamespace
        some-namespace-too:
          brokerClass: somenamespaceclasstoo
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
			ClusterDefault: &ClassAndBrokerConfig{
				BrokerClass: "clusterbrokerclass",
				BrokerConfig: &BrokerConfig{
					KReference: &duckv1.KReference{
						Kind:       "ConfigMap",
						APIVersion: "v1",
						Namespace:  "knative-eventing",
						Name:       "somename",
					},
					Delivery: nil,
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
      clusterDefault:
        brokerClass: clusterbrokerclass
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
			NamespaceDefaultsConfig: map[string]*ClassAndBrokerConfig{
				"some-namespace": {
					BrokerClass: "brokerclassnamespace",
					BrokerConfig: &BrokerConfig{
						KReference: &duckv1.KReference{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       "someothername",
							Namespace:  "someothernamespace",
						},
						Delivery: nil,
					},
				},
				"some-namespace-too": {
					BrokerClass: "brokerclassnamespacetoo",
					BrokerConfig: &BrokerConfig{
						KReference: &duckv1.KReference{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       "someothernametoo",
							Namespace:  "someothernamespacetoo",
						},
						Delivery: nil,
					},
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
          brokerClass: brokerclassnamespace
          apiVersion: v1
          kind: ConfigMap
          name: someothername
          namespace: someothernamespace
        some-namespace-too:
          brokerClass: brokerclassnamespacetoo
          apiVersion: v1
          kind: ConfigMap
          name: someothernametoo
          namespace: someothernamespacetoo
`,
			},
		},
	}, {
		name:    "only namespace config default, cluster brokerclass",
		wantErr: false,
		wantDefaults: &Defaults{
			ClusterDefault: &ClassAndBrokerConfig{
				BrokerClass: "clusterbrokerclass",
			},
			NamespaceDefaultsConfig: map[string]*ClassAndBrokerConfig{
				"some-namespace": {
					BrokerConfig: &BrokerConfig{
						KReference: &duckv1.KReference{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       "someothername",
							Namespace:  "someothernamespace",
						},
						Delivery: nil,
					},
				},
				"some-namespace-too": {
					BrokerConfig: &BrokerConfig{
						KReference: &duckv1.KReference{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       "someothernametoo",
							Namespace:  "someothernamespacetoo",
						},
						Delivery: nil,
					},
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
      clusterDefault:
        brokerClass: clusterbrokerclass
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
		name:    "one namespace config default, namespace config default with class, cluster brokerclass",
		wantErr: false,
		wantDefaults: &Defaults{
			ClusterDefault: &ClassAndBrokerConfig{
				BrokerClass: "clusterbrokerclass",
			},
			NamespaceDefaultsConfig: map[string]*ClassAndBrokerConfig{
				"some-namespace": {
					BrokerClass: "namespacebrokerclass",
					BrokerConfig: &BrokerConfig{
						KReference: &duckv1.KReference{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       "someothername",
							Namespace:  "someothernamespace",
						},
						Delivery: nil,
					},
				},
				"some-namespace-too": {
					BrokerConfig: &BrokerConfig{
						KReference: &duckv1.KReference{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       "someothernametoo",
							Namespace:  "someothernamespacetoo",
						},
						Delivery: nil,
					},
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
      clusterDefault:
        brokerClass: clusterbrokerclass
      namespaceDefaults:
        some-namespace:
          brokerClass: namespacebrokerclass
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
					t.Fatal("diff", diff)
				}
			}
		})
	}
}
