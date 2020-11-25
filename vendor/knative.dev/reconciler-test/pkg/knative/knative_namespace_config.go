/*
Copyright 2020 The Knative Authors

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

package knative

import (
	"context"

	"knative.dev/reconciler-test/pkg/environment"
)

type knativeNamespaceConfig struct{}

func WithKnativeNamespace(namespace string) environment.EnvOpts {
	return func(ctx context.Context, env environment.Environment) (context.Context, error) {
		return context.WithValue(ctx, knativeNamespaceConfig{}, namespace), nil
	}
}

func KnativeNamespaceFromContext(ctx context.Context) string {
	if e, ok := ctx.Value(knativeNamespaceConfig{}).(string); ok {
		return e
	}
	panic("no knative namespace found in the context, make sure you properly configured the env opts using WithKnativeNamespace")
}
