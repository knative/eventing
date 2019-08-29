/*
Copyright 2019 The Knative Authors

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

// Package logging is a copy of knative/pkg's logging package, except it uses desugared loggers.
package logging

import (
	"context"

	"go.uber.org/zap"
	"knative.dev/pkg/logging"
)

func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return logging.WithLogger(ctx, logger.Sugar())
}

func FromContext(ctx context.Context) *zap.Logger {
	return logging.FromContext(ctx).Desugar()
}

func With(ctx context.Context, fields ...zap.Field) context.Context {
	logger := FromContext(ctx)
	return WithLogger(ctx, logger.With(fields...))
}
