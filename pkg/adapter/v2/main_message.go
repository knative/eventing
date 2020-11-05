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
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/kelseyhightower/envconfig"
	"go.opencensus.io/stats/view"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/profiling"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/source"

	"knative.dev/eventing/pkg/kncloudevents"
)

type MessageAdapter interface {
	Start(ctx context.Context) error
}

type MessageAdapterConstructor func(ctx context.Context, env EnvConfigAccessor, adapter *kncloudevents.HTTPMessageSender, reporter source.StatsReporter) MessageAdapter

func MainMessageAdapter(component string, ector EnvConfigConstructor, ctor MessageAdapterConstructor) {
	ctx := signals.NewContext()

	MainMessageAdapterWithContext(ctx, component, ector, ctor)
}

func MainMessageAdapterWithContext(ctx context.Context, component string, ector EnvConfigConstructor, ctor MessageAdapterConstructor) {
	flag.Parse()

	env := ector()
	if err := envconfig.Process("", env); err != nil {
		log.Fatalf("Error processing env var: %s", err)
	}

	// Retrieve the logger from the env
	logger := env.GetLogger()
	defer flush(logger)
	ctx = logging.WithLogger(ctx, logger)

	// Report stats on Go memory usage every 30 seconds.
	msp := metrics.NewMemStatsAll()
	msp.Start(ctx, 30*time.Second)
	if err := view.Register(msp.DefaultViews()...); err != nil {
		logger.Fatal("Error exporting go memstats view: %v", zap.Error(err))
	}

	// Convert json metrics.ExporterOptions to metrics.ExporterOptions.
	metricsConfig, err := env.GetMetricsConfig()
	if err != nil {
		logger.Error("failed to process metrics options", zap.Error(err))
	} else {
		if err := metrics.UpdateExporter(ctx, *metricsConfig, logger); err != nil {
			logger.Error("failed to create the metrics exporter", zap.Error(err))
		}
	}

	// Check if metrics config contains profiling flag
	if metricsConfig != nil && metricsConfig.ConfigMap != nil {
		if enabled, err := profiling.ReadProfilingFlag(metricsConfig.ConfigMap); err == nil {
			if enabled {
				// Start a goroutine to server profiling metrics
				logger.Info("Profiling enabled")
				go func() {
					server := profiling.NewServer(profiling.NewHandler(logger, true))
					// Don't forward ErrServerClosed as that indicates we're already shutting down.
					if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
						logger.Error("profiling server failed", zap.Error(err))
					}
				}()
			}
		} else {
			logger.Error("error while reading profiling flag", zap.Error(err))
		}
	}

	reporter, err := source.NewStatsReporter()
	if err != nil {
		logger.Error("error building statsreporter", zap.Error(err))
	}

	// Retrieve tracing config
	if err := env.SetupTracing(logger); err != nil {
		// If tracing doesn't work, we will log an error, but allow the adapter
		// to continue to start.
		logger.Error("Error setting up trace publishing", zap.Error(err))
	}

	httpBindingsSender, err := kncloudevents.NewHTTPMessageSenderWithTarget(env.GetSink())
	if err != nil {
		logger.Fatal("error building cloud event client", zap.Error(err))
	}

	// Configuring the adapter
	adapter := ctor(ctx, env, httpBindingsSender, reporter)

	// Finally start the adapter (blocking)
	if err := adapter.Start(ctx); err != nil {
		logging.FromContext(ctx).Warn("Start returned an error", zap.Error(err))
	}
}
