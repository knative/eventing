/*
Copyright 2020 The Knative Authors

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

package adapter

import (
	"context"

	"knative.dev/pkg/configmap"
)

type haEnabledKey struct{}

// WithHAEnabled signals to MainWithContext that it should set up an appropriate leader elector for this component.
func WithHAEnabled(ctx context.Context) context.Context {
	return context.WithValue(ctx, haEnabledKey{}, struct{}{})
}

// IsHAEnabled checks the context for the desire to enable leader elector.
func IsHAEnabled(ctx context.Context) bool {
	val := ctx.Value(haEnabledKey{})
	return val != nil
}

type haDisabledFlagKey struct{}

// withHADisabledFlag signals to MainWithConfig that it should not set up an appropriate leader elector for this component.
func withHADisabledFlag(ctx context.Context) context.Context {
	return context.WithValue(ctx, haDisabledFlagKey{}, struct{}{})
}

// isHADisabledFlag checks the context for the desired to disable leader elector.
func isHADisabledFlag(ctx context.Context) bool {
	val := ctx.Value(haDisabledFlagKey{})
	return val != nil
}

type controllerKey struct{}

// WithController signals to MainWithContext that it should
// create and configure a controller notifying the adapter
// when a resource is ready and removed
func WithController(ctx context.Context, ctor ControllerConstructor) context.Context {
	return context.WithValue(ctx, controllerKey{}, ctor)
}

// ControllerFromContext gets the controller constructor from the context
func ControllerFromContext(ctx context.Context) ControllerConstructor {
	value := ctx.Value(controllerKey{})
	if value == nil {
		return nil
	}
	return value.(ControllerConstructor)
}

type namespaceKey struct{}

// WithNamespace defines the working namespace for the adapter.
func WithNamespace(ctx context.Context, namespace string) context.Context {
	return context.WithValue(ctx, namespaceKey{}, namespace)
}

// NamespaceFromContext gets the working namespace from the context
func NamespaceFromContext(ctx context.Context) string {
	value := ctx.Value(namespaceKey{})
	if value == nil {
		return ""
	}
	return value.(string)
}

type withConfigWatcherKey struct{}

// WithConfigWatcher adds a ConfigMap Watcher informer to the context.
func WithConfigWatcher(ctx context.Context, cmw configmap.Watcher) context.Context {
	return context.WithValue(ctx, withConfigWatcherKey{}, cmw)
}

// ConfigWatcherFromContext retrieves a ConfigMap Watcher from the context.
func ConfigWatcherFromContext(ctx context.Context) configmap.Watcher {
	if v := ctx.Value(withConfigWatcherKey{}); v != nil {
		return v.(configmap.Watcher)
	}
	return nil
}

type withConfigWatcherEnabledKey struct{}

// WithConfigWatcherEnabled flags the ConfigMapWatcher to be configured.
func WithConfigWatcherEnabled(ctx context.Context) context.Context {
	return context.WithValue(ctx, withConfigWatcherEnabledKey{}, struct{}{})
}

// IsConfigWatcherEnabled indicates whether the ConfigMapWatcher is required or not.
func IsConfigWatcherEnabled(ctx context.Context) bool {
	return ctx.Value(withConfigWatcherEnabledKey{}) != nil
}

type withConfiguratorOptions struct{}

// WithConfiguratorOptions sets custom options on the adapter configurator.
func WithConfiguratorOptions(ctx context.Context, opts []ConfiguratorOption) context.Context {
	return context.WithValue(ctx, withConfiguratorOptions{}, opts)
}

// ConfiguratorOptionsFromContext retrieves adapter configurator options.
func ConfiguratorOptionsFromContext(ctx context.Context) []ConfiguratorOption {
	value := ctx.Value(withConfiguratorOptions{})
	if value == nil {
		return nil
	}
	return value.([]ConfiguratorOption)
}
