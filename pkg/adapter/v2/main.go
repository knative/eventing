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

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/kelseyhightower/envconfig"
	"go.opencensus.io/stats/view"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/profiling"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/source"

	"knative.dev/eventing/pkg/tracing"
)

type Adapter interface {
	Start(stopCh <-chan struct{}) error
}

type AdapterConstructor func(ctx context.Context, env EnvConfigAccessor, client cloudevents.Client) Adapter

func Main(component string, ector EnvConfigConstructor, ctor AdapterConstructor) {
	MainWithContext(signals.NewContext(), component, ector, ctor)
}

func MainWithContext(ctx context.Context, component string, ector EnvConfigConstructor, ctor AdapterConstructor) {
	flag.Parse()

	env := ector()
	if err := envconfig.Process("", env); err != nil {
		log.Fatalf("Error processing env var: %s", err)
	}
	env.SetComponent(component)

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
	if metricsConfig, err := env.GetMetricsConfig(); err != nil {
		logger.Error("failed to process metrics options", zap.Error(err))
	} else {
		if err := metrics.UpdateExporter(*metricsConfig, logger); err != nil {
			logger.Error("failed to create the metrics exporter", zap.Error(err))
		}
		// Check if metrics config contains profiling flag
		if metricsConfig != nil && metricsConfig.ConfigMap != nil {
			if enabled, err := profiling.ReadProfilingFlag(metricsConfig.ConfigMap); err == nil && enabled {
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
		}
	}

	reporter, err := source.NewStatsReporter()
	if err != nil {
		logger.Error("error building statsreporter", zap.Error(err))
	}

	if err = tracing.SetupStaticPublishing(logger, "", tracing.OnePercentSampling); err != nil {
		// If tracing doesn't work, we will log an error, but allow the adapter
		// to continue to start.
		logger.Error("Error setting up trace publishing", zap.Error(err))
	}

	ceOverrides, err := env.GetCloudEventOverrides()
	if err != nil {
		logger.Error("Error loading cloudevents overrides", zap.Error(err))
	}

	eventsClient, err := NewCloudEventsClient(env.GetSink(), ceOverrides, reporter)
	if err != nil {
		logger.Fatal("error building cloud event client", zap.Error(err))
	}

	// Configuring the adapter
	adapter := ctor(ctx, env, eventsClient)

	logger.Info("Starting Receive Adapter", zap.Any("adapter", adapter))

	if err := adapter.Start(ctx.Done()); err != nil {
		logger.Warn("start returned an error", zap.Error(err))
	}
}

func flush(logger *zap.SugaredLogger) {
	_ = logger.Sync()
	metrics.FlushExporter()
}
