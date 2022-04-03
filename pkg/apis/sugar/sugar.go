/*
Copyright 2022 The Knative Authors

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

package sugar

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cm "knative.dev/pkg/configmap"
	"sigs.k8s.io/yaml"
)

const (
	// NamespaceSelectorKey is the name of the configuration
	// entry that specifies a LabelSelector to control which namespaces
	// the Sugar Controller operates on.
	NamespaceSelectorKey = "namespace-selector"

	// TriggerSelectorKey is the name of the configuration
	// entry that specifies a LabelSelector to control which triggers
	// the Sugar Controller operates on.
	TriggerSelectorKey = "trigger-selector"
)

// NewConfigFromConfigMap creates a Config from the supplied ConfigMap
func NewConfigFromConfigMap(configMap *corev1.ConfigMap) (*Config, error) {
	return NewConfigFromMap(configMap.Data)
}

// NewConfigFromMap creates a Config from the supplied data.
func NewConfigFromMap(data map[string]string) (*Config, error) {
	nc := &Config{}
	if err := cm.Parse(data,
		asLabelSelector(NamespaceSelectorKey, &nc.NamespaceSelector),
		asLabelSelector(TriggerSelectorKey, &nc.TriggerSelector),
	); err != nil {
		return nil, err
	}

	return nc, nil
}

// asLabelSelector returns a LabelSelector extracted from a given configmap key.
func asLabelSelector(key string, target **metav1.LabelSelector) cm.ParseFunc {
	return func(data map[string]string) error {
		if raw, ok := data[key]; ok {
			if len(raw) > 0 {
				var selector *metav1.LabelSelector
				if err := yaml.Unmarshal([]byte(raw), &selector); err != nil {
					return err
				}
				*target = selector
			}
		}
		return nil
	}
}
