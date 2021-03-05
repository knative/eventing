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

package state

import (
	"context"
)

type envKey struct{}

// ContextWith decorates the given context with the provided Store, and returns
// the resulting context.
func ContextWith(ctx context.Context, s Store) context.Context {
	return context.WithValue(ctx, envKey{}, s)
}

// FromContext returns the Store from Context, if not found FromContext will
// panic.
// TODO: revisit if we really want to panic here... likely not.
func FromContext(ctx context.Context) Store {
	if e, ok := ctx.Value(envKey{}).(Store); ok {
		return e
	}
	panic("no Store found in context")
}

// Fail is defined to avoid circular dependency with the feature package.
type fail interface {
	Error(args ...interface{})
}

// Get the string value from the kvstore from key.
func GetStringOrFail(ctx context.Context, t fail, key string) string {
	value := ""
	state := FromContext(ctx)
	if err := state.Get(ctx, key, &value); err != nil {
		t.Error(err)
	}
	return value
}

// Get gets the key from the Store into the provided value
func GetOrFail(ctx context.Context, t fail, key string, value interface{}) {
	state := FromContext(ctx)
	if err := state.Get(ctx, key, value); err != nil {
		t.Error(err)
	}
}

// Set sets the key into the Store from the provided value
func SetOrFail(ctx context.Context, t fail, key string, value interface{}) {
	state := FromContext(ctx)
	if err := state.Set(ctx, key, value); err != nil {
		t.Error(err)
	}
}
