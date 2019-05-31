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
	"github.com/knative/pkg/configmap"
	v1 "k8s.io/api/core/v1"
)

// TODO Move this to knative/pkg.

// DefaultConstructors is a map for specifying default ConfigMaps to their function constructors.
//
// The values of this map must be functions with the definition:
//
// func(*k8s.io/api/core/v1.ConfigMap) (... , error)
//
// These functions can return any type along with an error.
type DefaultConstructors map[*v1.ConfigMap]interface{}

// DefaultUntypedStore is an UntypedStore with default values for ConfigMaps that do not exist.
type DefaultUntypedStore struct {
	store      *configmap.UntypedStore
	defaultCMs []v1.ConfigMap
}

// NewDefaultUntypedStore creates a new DefaultUntypedStore.
func NewDefaultUntypedStore(
	name string,
	logger configmap.Logger,
	defaultConstructors DefaultConstructors,
	onAfterStore ...func(name string, value interface{})) *DefaultUntypedStore {
	constructors := configmap.Constructors{}
	defaultCMs := make([]v1.ConfigMap, 0, len(defaultConstructors))
	for cm, ctor := range defaultConstructors {
		constructors[cm.Name] = ctor
		defaultCMs = append(defaultCMs, *cm)
	}
	return &DefaultUntypedStore{
		store:      configmap.NewUntypedStore(name, logger, constructors, onAfterStore...),
		defaultCMs: defaultCMs,
	}
}

// WatchConfigs uses the provided configmap.DefaultingWatcher to setup watches for the config maps
// provided in defaultCMs.
func (s *DefaultUntypedStore) WatchConfigs(w configmap.DefaultingWatcher) {
	for _, cm := range s.defaultCMs {
		w.WatchWithDefault(cm, s.store.OnConfigChanged)
	}
}
