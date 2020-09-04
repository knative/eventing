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
