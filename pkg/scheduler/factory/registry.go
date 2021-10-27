/*
Copyright 2021 The Knative Authors

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

package factory

import (
	"fmt"

	state "knative.dev/eventing/pkg/scheduler/state"
)

// RegistryFP is a collection of all available filter plugins.
type RegistryFP map[string]state.FilterPlugin

// RegistrySP is a collection of all available scoring plugins.
type RegistrySP map[string]state.ScorePlugin

var (
	FilterRegistry = make(RegistryFP)
	ScoreRegistry  = make(RegistrySP)
)

// Register adds a new plugin to the registry. If a plugin with the same name
// exists, it returns an error.
func RegisterFP(name string, factory state.FilterPlugin) error {
	if _, ok := FilterRegistry[name]; ok {
		return fmt.Errorf("a filter plugin named %v already exists", name)
	}
	FilterRegistry[name] = factory
	return nil
}

// Unregister removes an existing plugin from the registry. If no plugin with
// the provided name exists, it returns an error.
func UnregisterFP(name string) error {
	if _, ok := FilterRegistry[name]; !ok {
		return fmt.Errorf("no filter plugin named %v exists", name)
	}
	delete(FilterRegistry, name)
	return nil
}

func GetFilterPlugin(name string) (state.FilterPlugin, error) {
	if f, exist := FilterRegistry[name]; exist {
		return f, nil
	}
	return nil, fmt.Errorf("no fitler plugin named %v exists", name)
}

// Register adds a new plugin to the registry. If a plugin with the same name
// exists, it returns an error.
func RegisterSP(name string, factory state.ScorePlugin) error {
	if _, ok := ScoreRegistry[name]; ok {
		return fmt.Errorf("a score plugin named %v already exists", name)
	}
	ScoreRegistry[name] = factory
	return nil
}

// Unregister removes an existing plugin from the registry. If no plugin with
// the provided name exists, it returns an error.
func UnregisterSP(name string) error {
	if _, ok := ScoreRegistry[name]; !ok {
		return fmt.Errorf("no score plugin named %v exists", name)
	}
	delete(ScoreRegistry, name)
	return nil
}

func GetScorePlugin(name string) (state.ScorePlugin, error) {
	if f, exist := ScoreRegistry[name]; exist {
		return f, nil
	}
	return nil, fmt.Errorf("no score plugin named %v exists", name)
}
