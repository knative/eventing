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

package logging

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"knative.dev/pkg/logging"
)

// NewContext returns a context with test logger configured. This is interim
// logger, that should be replaced by testing.T bound test logger using
// environment.WithTestLogger func.
func NewContext(parent ...context.Context) context.Context {
	if len(parent) > 1 {
		panic("pass 0 or 1 context.Context while creating context")
	}
	var ctx context.Context
	if len(parent) == 0 {
		ctx = context.TODO()
	} else {
		ctx = parent[0]
	}
	level := LevelFromEnvironment(ctx)
	log := defaultLogger(level)
	if len(parent) == 0 {
		log.Warn("Using context.TODO() as no real context was provided")
	}
	return logging.WithLogger(ctx, log)
}

// WithTestLogger returns a context with test logger configured.
func WithTestLogger(ctx context.Context, t zaptest.TestingT, opts ...zaptest.LoggerOption) context.Context {
	opts = append([]zaptest.LoggerOption{
		zaptest.Level(LevelFromEnvironment(ctx)),
		zaptest.WrapOptions(zap.AddCaller(), zap.Fields(
			zap.String("test", t.Name()),
		)),
	}, opts...)
	log := zaptest.NewLogger(t, opts...)
	return logging.WithLogger(ctx, log.Sugar())
}

func defaultLogger(level zapcore.Level) *zap.SugaredLogger {
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(level)
	if log, err := config.Build(); err != nil {
		panic(err)
	} else {
		return log.Named("test").Sugar()
	}
}
