/*
Copyright 2021 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package eventshub

import (
	"context"

	"knative.dev/reconciler-test/pkg/environment"
)

const (
	thisPackage           = "knative.dev/reconciler-test/cmd/eventshub"
	defaultEventshubImage = "ko://" + thisPackage
)

type eventshubImageKey struct{}

// ImageFromContext gets the eventshub image from context
func ImageFromContext(ctx context.Context) string {
	if e, ok := ctx.Value(eventshubImageKey{}).(string); ok {
		return e
	}
	return defaultEventshubImage
}

// WithCustomImage allows you to specify a custom eventshub image to be used when invoking eventshub.Install
func WithCustomImage(image string) environment.EnvOpts {
	return func(ctx context.Context, env environment.Environment) (context.Context, error) {
		return context.WithValue(ctx, eventshubImageKey{}, image), nil
	}
}
