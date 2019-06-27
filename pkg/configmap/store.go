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

package configmap

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/configmap"
)

// TODO Move this to knative/pkg.

// DefaultConstructor defines a default ConfigMap to use if the real ConfigMap does not exist and
// the constructor to use to parse both the default and any real ConfigMap with that name.
type DefaultConstructor struct {
	// Default is the default value to use for the ConfigMap if a real one does not exist. Its name
	// is used to determine which ConfigMap to watch.
	Default v1.ConfigMap
	// Constructor follows the same interface as configmap.DefaultConstructor's value.
	Constructor interface{}
}

// DefaultUntypedStore is an UntypedStore with default values for ConfigMaps that do not exist.
type DefaultUntypedStore struct {
	store      *configmap.UntypedStore
	defaultCMs []v1.ConfigMap
}

// NewDefaultUntypedStore creates a new DefaultUntypedStore.
func NewDefaultUntypedStore(
	name string,
	logger configmap.Logger,
	defaultConstructors []DefaultConstructor,
	onAfterStore ...func(name string, value interface{})) *DefaultUntypedStore {
	constructors := configmap.Constructors{}
	defaultCMs := make([]v1.ConfigMap, 0, len(defaultConstructors))
	for _, dc := range defaultConstructors {
		cmName := dc.Default.Name
		if _, present := constructors[cmName]; present {
			panic(fmt.Sprintf("tried to add ConfigMap named %q more than once", cmName))
		}
		constructors[cmName] = dc.Constructor
		defaultCMs = append(defaultCMs, dc.Default)
	}
	return &DefaultUntypedStore{
		store:      configmap.NewUntypedStore(name, logger, constructors, onAfterStore...),
		defaultCMs: defaultCMs,
	}
}

// WatchConfigs uses the provided configmap.DefaultingWatcher to setup watches for the config maps
// provided in defaultCMs.
func (s *DefaultUntypedStore) WatchConfigs(w configmap.Watcher) {
	for _, cm := range s.defaultCMs {
		w.Watch(cm.Name, s.store.OnConfigChanged)
	}
}
