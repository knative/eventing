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

package feature

import (
	"context"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/configmap"
)

const (
	// FlagsConfigName is the name of config map containing the experimental features flags
	FlagsConfigName = "config-features"
)

type cfgKey struct{}

// FromContext extracts a Config from the provided context.
func FromContext(ctx context.Context) Flags {
	x, ok := ctx.Value(cfgKey{}).(Flags)
	if ok {
		return x
	}
	return nil
}

// FromContextOrDefaults is like FromContext, but when no Flags is attached it
// returns default Flags.
func FromContextOrDefaults(ctx context.Context) Flags {
	if cfg := FromContext(ctx); cfg != nil {
		return cfg
	}
	return newDefaults()
}

// ToContext attaches the provided Flags to the provided context, returning the
// new context with the Flags attached.
func ToContext(ctx context.Context, c Flags) context.Context {
	return fillContextWithFeatureSpecificFlags(context.WithValue(ctx, cfgKey{}, c), c)
}

// Store is a typed wrapper around configmap.Untyped store to handle our configmaps.
// +k8s:deepcopy-gen=false
type Store struct {
	*configmap.UntypedStore
}

// NewStore creates a new store of Configs and optionally calls functions when ConfigMaps are updated.
func NewStore(logger configmap.Logger, onAfterStore ...func(name string, value interface{})) *Store {
	store := &Store{
		UntypedStore: configmap.NewUntypedStore(
			"feature-flags",
			logger,
			configmap.Constructors{
				FlagsConfigName: NewFlagsConfigFromConfigMap,
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

// IsEnabled is a shortcut for Load().IsEnabled(featureName)
func (s *Store) IsEnabled(featureName string) bool {
	return s.Load().IsEnabled(featureName)
}

// IsAllowed is a shortcut for Load().IsAllowed(featureName)
func (s *Store) IsAllowed(featureName string) bool {
	return s.Load().IsAllowed(featureName)
}

// Load creates a Config from the current config state of the Store.
func (s *Store) Load() Flags {
	loaded := s.UntypedLoad(FlagsConfigName)
	if loaded == nil {
		return Flags(nil)
	}
	return loaded.(Flags)
}

func fillContextWithFeatureSpecificFlags(ctx context.Context, flags Flags) context.Context {
	if flags.IsEnabled(KReferenceGroup) {
		ctx = duckv1.KReferenceGroupAllowed(ctx)
	}
	return ctx
}
