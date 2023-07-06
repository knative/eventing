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

package tracing

import (
	"context"
	"log"

	"go.uber.org/zap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/test/zipkin"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/knative"
	"knative.dev/reconciler-test/pkg/milestone"
)

// WithGatherer registers the trace gatherer with the environment and will be
// receiving milestone events.
func WithGatherer(t feature.T) environment.EnvOpts {
	return func(ctx context.Context, env environment.Environment) (context.Context, error) {
		gatherer, err := milestone.NewTracingGatherer(ctx, env.Namespace(),
			knative.KnativeNamespaceFromContext(ctx), t)
		if err == nil {
			ctx, err = environment.WithEmitter(gatherer)(ctx, env)
		}
		if err != nil {
			logging.FromContext(ctx).Error("failed to create tracing gatherer", zap.Error(err))
		}
		return ctx, nil
	}
}

func Cleanup() {
	zipkin.CleanupZipkinTracingSetup(log.Printf)
}
