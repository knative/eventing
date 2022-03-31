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
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/configmap"
)

type sugarCfgKey struct{}

const ConfigName = "config-sugar"

// Config holds the collection of configurations that we attach to contexts.
type Config struct {
	// NamespaceSelector specifies a LabelSelector which
	// determines which namespaces the Sugar Controller should operate upon
	NamespaceSelector *metav1.LabelSelector

	// TriggerSelector specifies a LabelSelector which
	// determines which triggers the Sugar Controller should operate upon
	TriggerSelector *metav1.LabelSelector
}

func (c *Config) DeepCopy() *Config {
	if c == nil {
		return nil
	}
	out := new(Config)
	*out = *c
	return out
}

// FromContext extracts a Config from the provided context.
func FromContext(ctx context.Context) *Config {
	c, ok := ctx.Value(sugarCfgKey{}).(*Config)
	if ok {
		return c
	}
	return nil
}

// ToContext adds config to given context.
func ToContext(ctx context.Context, c *Config) context.Context {
	return context.WithValue(ctx, sugarCfgKey{}, c)
}

// Store is a typed wrapper around configmap.Untyped store to handle our configmaps.
type Store struct {
	*configmap.UntypedStore
}

// NewStore creates a new store of Configs and optionally calls functions when ConfigMaps are updated.
func NewStore(logger configmap.Logger, onAfterStore ...func(name string, value interface{})) *Store {
	store := &Store{
		UntypedStore: configmap.NewUntypedStore(
			"sugar",
			logger,
			configmap.Constructors{
				ConfigName: NewConfigFromConfigMap,
			},
			onAfterStore...,
		),
	}

	return store
}

// ToContext attaches the current Config state to the provided context.
func (s *Store) ToContext(ctx context.Context) context.Context {
	return ToContext(ctx, s.Load())
}

// Load creates a Config from the current config state of the Store.
func (s *Store) Load() *Config {
	return s.UntypedLoad(ConfigName).(*Config).DeepCopy()
}
