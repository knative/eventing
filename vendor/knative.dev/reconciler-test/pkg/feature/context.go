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
)

type envKey struct{}

// ContextWith decorates the given context with the provided Feature, and returns
// the resulting context.
func ContextWith(ctx context.Context, f *Feature) context.Context {
	return context.WithValue(ctx, envKey{}, f)
}

// FromContext returns the Feature from Context, if not found FromContext will
// panic.
// TODO: revisit if we really want to panic here... likely not.
func FromContext(ctx context.Context) *Feature {
	if e, ok := ctx.Value(envKey{}).(*Feature); ok {
		return e
	}
	panic("no Feature found in context")
}
