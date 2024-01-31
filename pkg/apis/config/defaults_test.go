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

	// Test cluster default, without given broker class name
	// Will use cluster default broker class, as no default namespace config for ns "rando"
	// The name should be config-default-cluster-class
	c, err := defaults.GetBrokerConfig("rando", "")
	if err != nil {
		t.Error("GetBrokerConfig Failed =", err)
	}
	if c.Name != "config-default-cluster-class" {
		t.Error("GetBrokerConfig Failed, wanted config-default-cluster-class, got:", c.Name)
	}

	// Test cluster default, with given broker class name: cluster-class-2
	// Will use the given broker class name with the correspond config from cluster brokerClasses, as no default namespace config for ns "rando"
	// The name for the config should be config-cluster-class-2
	c, err = defaults.GetBrokerConfig("rando", "cluster-class-2")
	if err != nil {
		t.Error("GetBrokerConfig Failed =", err)
	}
	if c.Name != "config-cluster-class-2" {
		t.Error("GetBrokerConfig Failed, wanted config-cluster-class-2, got:", c.Name)
	}

	// Test namespace default, without given broker class name
	// Will use the namespace default broker class, and the config exist for this namespace
	// The config name should be config-default-namespace-1-class
	c, err = defaults.GetBrokerConfig("namespace-1", "")
	if err != nil {
		t.Error("GetBrokerConfig Failed =", err)
	}
	if c.Name != "config-namespace-1-class" {
		t.Error("GetBrokerConfig Failed, wanted config-namespace-1-class, got:", c.Name)
	}

	// Test namespace default, with given broker class name: namespace-1-class-2
	// Will use the given broker class name with the correspond config from namespace brokerClasses
	// The config name should be config-namespace-1-class-2
	c, err = defaults.GetBrokerConfig("namespace-1", "namespace-1-class-2")
	if err != nil {
		t.Error("GetBrokerConfig Failed =", err)
	}
	if c.Name != "config-namespace-1-class-2" {
		t.Error("GetBrokerConfig Failed, wanted config-namespace-1-class-2, got:", c.Name)
	}

	// Test namespace default, with given broker class name that doesn't have config in this namespace's brokerClasses
	// Will use the cluster config for this broker class. i.e will looking for the config in cluster brokerClasses
	// The config name should be config-cluster-class-2
	c, err = defaults.GetBrokerConfig("namespace-1", "cluster-class-2")
	if err != nil {
		t.Error("GetBrokerConfig Failed =", err)
	}
	if c.Name != "config-cluster-class-2" {
		t.Error("GetBrokerConfig Failed, wanted config-cluster-class-2, got:", c.Name)
	}

	// Test namespace default, specify a broker class name that doesn't exist in this namespace's brokerClasses, but it is cluster's default broker class
	// Will use the cluster default broker class config, as no default broker class config is set for this namespace
	// The config name should be config-default-cluster-class
	c, err = defaults.GetBrokerConfig("namespace-1", "default-cluster-class")
	if err != nil {
		t.Error("GetBrokerConfig Failed =", err)
	}
	if c.Name != "config-default-cluster-class" {
		t.Error("GetBrokerConfig Failed, wanted config-default-cluster-class, got:", c.Name)
	}

	// Shared config
	// Test namespace default, with given broker class name both have config in this namespace's brokerClasses and cluster brokerClasses
	// Will use the given broker class name with the correspond config from namespace brokerClasses. i.e namespace will override cluster config
	// The config name should be config-shared-class-in-namespace-1
	c, err = defaults.GetBrokerConfig("namespace-1", "shared-class")
	if err != nil {
		t.Error("GetBrokerConfig Failed =", err)
	}
	if c.Name != "config-shared-class-in-namespace-1" {
		t.Error("GetBrokerConfig Failed, wanted config-shared-class-in-namespace-1, got:", c.Name)
	}

	// Test namespace default, with given broker class name that doesn't have config in this namespace's brokerClasses, and also doesn't have config in cluster brokerClasses
	// Should return the cluster default
	// The config name should be config-default-cluster-class
	c, err = defaults.GetBrokerConfig("namespace-1", "cluster-class-3")
	if err != nil {
		t.Error("GetBrokerConfig Failed =", err)
	}
	if c.Name != "config-default-cluster-class" {
		t.Error("GetBrokerConfig Failed, wanted config-default-cluster-class, got:", c.Name)
	}

	// Test namespace default, without specifying broker class name, and the namespace default broker class's config is not set
	// Will use the cluster default broker class config, as no default broker class config is set for this namespace
	// The config name should be config-default-cluster-class
	c, err = defaults.GetBrokerConfig("namespace-2", "")
	if err != nil {
		t.Error("GetBrokerConfig Failed =", err)
	}
	if c.Name != "config-default-cluster-class" {
		t.Error("GetBrokerConfig Failed, wanted config-default-cluster-class, got:", c.Name)
	}

	// Test namespace default, without specifying broker class name, and nothing for this namespace is set
	// Will use the cluster default broker class config, as nothing is setted for this namespace
	// The config name should be config-default-cluster-class
	c, err = defaults.GetBrokerConfig("namespace-3", "")
	if err != nil {
		t.Error("GetBrokerConfig Failed =", err)
	}
	if c.Name != "config-default-cluster-class" {
		t.Error("GetBrokerConfig Failed, wanted config-default-cluster-class, got:", c.Name)
	}

	// Nil and empty tests
	var nilDefaults *Defaults
	_, err = nilDefaults.GetBrokerConfig("rando", "")
	if err == nil {
		t.Errorf("GetBrokerConfig did not fail with nil")
	}
	if err.Error() != "Defaults are nil" {
		t.Error("GetBrokerConfig did not fail with nil msg, got", err)
	}

	emptyDefaults := Defaults{}
	_, err = emptyDefaults.GetBrokerConfig("rando", "")
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

	// Test cluster default, the given namespace doesn't have default broker class
	// Will use the cluster default broker class
	// The class should be clusterbrokerclass
	c, err := defaults.GetBrokerClass("rando")
	if err != nil {
		t.Error("GetBrokerClass Failed =", err)
	}
	if c != "default-cluster-class" {
		t.Error("GetBrokerClass Failed, wanted default-cluster-class, got:", c)
	}

	// Test namespace default, the given namespace has default broker class
	c, err = defaults.GetBrokerClass("namespace-1")
	if err != nil {
		t.Error("GetBrokerClass Failed =", err)
	}
	if c != "namespace-1-class" {
		t.Error("GetBrokerClass Failed, wanted namespace-1-class, got:", c)
	}

	// Nil and empty tests
	var nilDefaults *Defaults
	_, err = nilDefaults.GetBrokerClass("rando")
	if err == nil {
		t.Errorf("GetBrokerClass did not fail with nil")
	}
	if err.Error() != "Defaults are nil" {
		t.Error("GetBrokerClass did not fail with nil msg, got", err.Error())
	}

	emptyDefaults := Defaults{}
	_, err = emptyDefaults.GetBrokerClass("rando")
	if err == nil {
		t.Errorf("GetBrokerClass did not fail with empty")
	}
	if err.Error() != "Defaults are nil" {
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
		name:    "corrupt default-br-config",
		wantErr: true,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      DefaultsConfigName,
			},
			Data: map[string]string{
				"default-br-config": `
     broken YAML
`,
			},
		},
	}, {
		name:    "all specified values",
		wantErr: false,
		wantDefaults: &Defaults{
			NamespaceDefaultsConfig: map[string]*DefaultConfig{
				"some-namespace": {
					DefaultBrokerClass: "somenamespaceclass",
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
					DefaultBrokerClass: "somenamespaceclasstoo",
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
			ClusterDefaultConfig: &DefaultConfig{
				DefaultBrokerClass: "clusterbrokerclass",
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
			ClusterDefaultConfig: &DefaultConfig{
				DefaultBrokerClass: "clusterbrokerclass",
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
			NamespaceDefaultsConfig: map[string]*DefaultConfig{
				"some-namespace": {
					DefaultBrokerClass: "brokerclassnamespace",
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
					DefaultBrokerClass: "brokerclassnamespacetoo",
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
			ClusterDefaultConfig: &DefaultConfig{
				DefaultBrokerClass: "clusterbrokerclass",
			},
			NamespaceDefaultsConfig: map[string]*DefaultConfig{
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
			ClusterDefaultConfig: &DefaultConfig{
				DefaultBrokerClass: "clusterbrokerclass",
			},
			NamespaceDefaultsConfig: map[string]*DefaultConfig{
				"some-namespace": {
					DefaultBrokerClass: "namespacebrokerclass",
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
