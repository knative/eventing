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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	logtesting "knative.dev/pkg/logging/testing"

	. "knative.dev/pkg/configmap/testing"
)

func TestStoreLoadWithContext(t *testing.T) {
	store := NewStore(logtesting.TestLogger(t))

	_, defaultsConfig := ConfigMapsFromTestFile(t, DefaultsConfigName)

	store.OnConfigChanged(defaultsConfig)

	config := FromContextOrDefaults(store.ToContext(context.Background()))

	t.Run("defaults", func(t *testing.T) {
		expected, _ := NewDefaultsConfigFromConfigMap(defaultsConfig)
		if diff := cmp.Diff(expected, config.Defaults, ignoreStuff...); diff != "" {
			t.Errorf("Unexpected defaults config (-want, +got): %v", diff)
			t.Fatalf("Unexpected defaults config (-want, +got): %v", diff)
		}
	})
}

func TestStoreLoadWithContextOrDefaults(t *testing.T) {
	defaultsConfig := ConfigMapFromTestFile(t, DefaultsConfigName)
	config := FromContextOrDefaults(context.Background())

	t.Run("defaults", func(t *testing.T) {
		expected, _ := NewDefaultsConfigFromConfigMap(defaultsConfig)
		if diff := cmp.Diff(expected, config.Defaults, ignoreStuff...); diff != "" {
			t.Errorf("Unexpected defaults config (-want, +got): %v", diff)
		}
	})
}
