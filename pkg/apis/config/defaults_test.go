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
		t.Fatal("NewDefaultsConfigFromConfigMap(example) failed:", err)
	}

	tests := []struct {
		name            string
		namespace       string
		brokerClassName *string
		wantConfigName  string
		wantErrMessage  string
	}{
		{
			name:           "Cluster default without broker class name",
			namespace:      "rando",
			wantConfigName: "config-default-cluster-class",
		},
		{
			name:            "Cluster default with given broker class name",
			namespace:       "rando",
			brokerClassName: stringPtr("cluster-class-2"),
			wantConfigName:  "config-cluster-class-2",
		},
		{
			name:           "Namespace default without broker class name",
			namespace:      "namespace-1",
			wantConfigName: "config-namespace-1-class",
		},
		{
			name:            "Namespace default with given broker class name: namespace-1-class-2",
			namespace:       "namespace-1",
			brokerClassName: stringPtr("namespace-1-class-2"),
			wantConfigName:  "config-namespace-1-class-2",
		},
		{
			name:            "Namespace default with given broker class name that doesn't have config but cluster does have",
			namespace:       "namespace-1",
			brokerClassName: stringPtr("cluster-class-2"),
			wantConfigName:  "config-cluster-class-2",
		},
		{
			name:            "Namespace default, specify a broker class name that is cluster's default",
			namespace:       "namespace-1",
			brokerClassName: stringPtr("default-cluster-class"),
			wantConfigName:  "config-default-cluster-class",
		},
		{
			name:            "Shared config in namespace overrides cluster",
			namespace:       "namespace-1",
			brokerClassName: stringPtr("shared-class"),
			wantConfigName:  "config-shared-class-in-namespace-1",
		},
		{
			name:            "Namespace default with non-existent broker class name",
			namespace:       "namespace-1",
			brokerClassName: stringPtr("cluster-class-3"),
			wantConfigName:  "config-default-cluster-class",
		},
		{
			name:           "Namespace without specifying broker class name, default config not set",
			namespace:      "namespace-2",
			wantConfigName: "config-default-cluster-class",
		},
		{
			name:           "Namespace without specifying broker class name, nothing set for namespace",
			namespace:      "namespace-3",
			wantConfigName: "config-default-cluster-class",
		},
		{
			name:           "Nil Defaults",
			wantErrMessage: "Defaults are nil",
		},
		{
			name:           "Empty Defaults",
			namespace:      "rando",
			wantErrMessage: "Defaults for Broker Configurations have not been set up.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c *BrokerConfig
			var err error

			// Special handling for the nil and empty defaults test cases
			if tt.wantErrMessage != "" {
				if tt.wantErrMessage == "Defaults are nil" {
					var nilDefaults *Defaults
					c, err = nilDefaults.GetBrokerConfig(tt.namespace, tt.brokerClassName)
				} else {
					emptyDefaults := Defaults{}
					c, err = emptyDefaults.GetBrokerConfig(tt.namespace, tt.brokerClassName)
				}
			} else {
				c, err = defaults.GetBrokerConfig(tt.namespace, tt.brokerClassName)
			}

			// Check for expected errors
			if tt.wantErrMessage != "" {
				if err == nil || err.Error() != tt.wantErrMessage {
					t.Errorf("Expected error %q, got %q", tt.wantErrMessage, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if c.Name != tt.wantConfigName {
				t.Errorf("Got config name %q, want %q", c.Name, tt.wantConfigName)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
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
					BrokerClasses: map[string]*BrokerConfig{
						"somensbrokerclass": {
							KReference: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "ConfigMap",
								Name:       "someothernsname",
								Namespace:  "somenamespace",
							},
						},
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
				BrokerClasses: map[string]*BrokerConfig{
					"somebrokerclass": {
						KReference: &duckv1.KReference{
							Kind:       "ConfigMap",
							APIVersion: "v1",
							Namespace:  "someothernamespace",
							Name:       "someothername",
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
  apiVersion: v1
  kind: ConfigMap
  name: somename
  namespace: knative-eventing
  brokerClasses:
    somebrokerclass:
      apiVersion: v1
      kind: ConfigMap
      name: someothername
      namespace: someothernamespace
namespaceDefaults:
  some-namespace:
    brokerClass: somenamespaceclass
    apiVersion: v1
    kind: ConfigMap
    name: someothername
    namespace: someothernamespace
    brokerClasses:
      somensbrokerclass:
        apiVersion: v1
        kind: ConfigMap
        name: someothernsname
        namespace: somenamespace
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
		name:    "all specified values with multiple brokerClasses",
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
					BrokerClasses: map[string]*BrokerConfig{
						"somensbrokerclass": {
							KReference: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "ConfigMap",
								Name:       "someothernsname",
								Namespace:  "somenamespace",
							},
						},
						"somebrokerclass2": {
							KReference: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "ConfigMap",
								Name:       "someothername",
								Namespace:  "someothernamespace",
							},
						},
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
					BrokerClasses: map[string]*BrokerConfig{
						"somensbrokerclass": {
							KReference: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "ConfigMap",
								Name:       "someothernsname",
								Namespace:  "somenamespace",
							},
						},
						"somebrokerclass2": {
							KReference: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "ConfigMap",
								Name:       "someothername",
								Namespace:  "someothernamespace",
							},
						},
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
				BrokerClasses: map[string]*BrokerConfig{
					"somebrokerclass": {
						KReference: &duckv1.KReference{
							Kind:       "ConfigMap",
							APIVersion: "v1",
							Namespace:  "someothernamespace",
							Name:       "someothername",
						},
						Delivery: nil,
					},
					"somebrokerclass2": {
						KReference: &duckv1.KReference{
							Kind:       "ConfigMap",
							APIVersion: "v1",
							Namespace:  "someothernamespace",
							Name:       "someothername",
						},
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
  apiVersion: v1
  kind: ConfigMap
  name: somename
  namespace: knative-eventing
  brokerClasses:
    somebrokerclass:
      apiVersion: v1
      kind: ConfigMap
      name: someothername
      namespace: someothernamespace
    somebrokerclass2:
      apiVersion: v1
      kind: ConfigMap
      name: someothername
      namespace: someothernamespace
namespaceDefaults:
  some-namespace:
    brokerClass: somenamespaceclass
    apiVersion: v1
    kind: ConfigMap
    name: someothername
    namespace: someothernamespace
    brokerClasses:
      somensbrokerclass:
        apiVersion: v1
        kind: ConfigMap
        name: someothernsname
        namespace: somenamespace
      somebrokerclass2:
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
    brokerClasses:
      somensbrokerclass:
        apiVersion: v1
        kind: ConfigMap
        name: someothernsname
        namespace: somenamespace
      somebrokerclass2:
        apiVersion: v1
        kind: ConfigMap
        name: someothername
        namespace: someothernamespace
`,
			},
		},
	}, {
		name:    "only clusterdefault specified values without brokerClasses",
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
				BrokerClasses: map[string]*BrokerConfig{
					"somebrokerclass": {
						KReference: &duckv1.KReference{
							Kind:       "ConfigMap",
							APIVersion: "v1",
							Namespace:  "someothernamespace",
							Name:       "someothername",
						},
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
  apiVersion: v1
  kind: ConfigMap
  name: somename
  namespace: knative-eventing
  brokerClasses:
    somebrokerclass:
      apiVersion: v1
      kind: ConfigMap
      name: someothername
      namespace: someothernamespace
`,
			},
		},
	}, {
		name:    "only namespace defaults without brokerClasses",
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
					BrokerClasses: map[string]*BrokerConfig{
						"somebrokerclass": {
							KReference: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "ConfigMap",
								Name:       "someothername",
								Namespace:  "someothernamespace",
							},
						},
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
    brokerClasses:
      somebrokerclass:
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
					BrokerClasses: map[string]*BrokerConfig{
						"somebrokerclass": {
							KReference: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "ConfigMap",
								Name:       "someothername",
								Namespace:  "someothernamespace",
							},
						},
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
					BrokerClasses: map[string]*BrokerConfig{
						"somebrokerclass": {
							KReference: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "ConfigMap",
								Name:       "someothername",
								Namespace:  "someothernamespace",
							},
						},
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
    brokerClasses:
      somebrokerclass:
        apiVersion: v1
        kind: ConfigMap
        name: someothername
        namespace: someothernamespace
  some-namespace-too:
    apiVersion: v1
    kind: ConfigMap
    name: someothernametoo
    namespace: someothernamespacetoo
    brokerClasses:
      somebrokerclass:
        apiVersion: v1
        kind: ConfigMap
        name: someothername
        namespace: someothernamespace
`,
			},
		},
	}, {
		name:    "only namespace config default, cluster brokerclass, with multiple brokerClasses",
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
					BrokerClasses: map[string]*BrokerConfig{
						"somebrokerclass": {
							KReference: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "ConfigMap",
								Name:       "someothername",
								Namespace:  "someothernamespace",
							},
						},
						"somebrokerclass2": {
							KReference: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "ConfigMap",
								Name:       "someothername",
								Namespace:  "someothernamespace",
							},
						},
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
					BrokerClasses: map[string]*BrokerConfig{
						"somebrokerclass": {
							KReference: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "ConfigMap",
								Name:       "someothername",
								Namespace:  "someothernamespace",
							},
						},
						"somebrokerclass2": {
							KReference: &duckv1.KReference{
								APIVersion: "v1",
								Kind:       "ConfigMap",
								Name:       "someothername",
								Namespace:  "someothernamespace",
							},
						},
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
    brokerClasses:
      somebrokerclass:
        apiVersion: v1
        kind: ConfigMap
        name: someothername
        namespace: someothernamespace
      somebrokerclass2:
        apiVersion: v1
        kind: ConfigMap
        name: someothername
        namespace: someothernamespace
  some-namespace-too:
    apiVersion: v1
    kind: ConfigMap
    name: someothernametoo
    namespace: someothernamespacetoo
    brokerClasses:
      somebrokerclass:
        apiVersion: v1
        kind: ConfigMap
        name: someothername
        namespace: someothernamespace
      somebrokerclass2:
        apiVersion: v1
        kind: ConfigMap
        name: someothername
        namespace: someothernamespace
`,
			},
		},
	}, {
		name:    "one namespace config default, namespace config default with class, cluster brokerclass, without brokerClasses",
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
	}, {
		name:    "complex broken YAML structure",
		wantErr: true,
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
    - invalid yaml
    namespaceDefaults:
      some-namespace:
        brokerClass: somenamespaceclass`,
			},
		},
	}, {
		// FIXME: Wondering that how should we handle the empty string in critical fields
		name:    "empty strings in critical fields",
		wantErr: false,
		wantDefaults: &Defaults{
			ClusterDefaultConfig: &DefaultConfig{
				DefaultBrokerClass: "",
			},
			NamespaceDefaultsConfig: map[string]*DefaultConfig{
				"some-namespace": {
					DefaultBrokerClass: "",
					BrokerConfig: &BrokerConfig{
						KReference: &duckv1.KReference{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       "",
							Namespace:  "",
						},
						Delivery: nil,
					},
				},
				"some-namespace-too": {
					BrokerConfig: &BrokerConfig{
						KReference: &duckv1.KReference{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       "",
							Namespace:  "",
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
       brokerClass:
     namespaceDefaults:
       some-namespace:
         brokerClass:
         apiVersion: v1
         kind: ConfigMap
         name:
         namespace:
       some-namespace-too:
         apiVersion: v1
         kind: ConfigMap
         name:
         namespace:
`,
			},
		},
	},
	}

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
