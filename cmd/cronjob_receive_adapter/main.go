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

package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/kelseyhightower/envconfig"
	"go.opencensus.io/stats/view"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/tracing"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/source"

	"knative.dev/eventing/pkg/adapter"
	"knative.dev/eventing/pkg/adapter/cronjobevents"
	"knative.dev/eventing/pkg/kncloudevents"
)

type envConfig struct {
	adapter.EnvConfig

	// Environment variable container schedule.
	Schedule string `envconfig:"SCHEDULE" required:"true"`

	// Environment variable containing data.
	Data string `envconfig:"DATA" required:"true"`

	// Environment variable containing the name of the cron job.
	Name string `envconfig:"NAME" required:"true"`
}

const (
	component = "cronjobsource"
)

func main() {
	flag.Parse()

	ctx := signals.NewContext()

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatalf("Error processing env var: %s", err)
	}

	// Report stats on Go memory usage every 30 seconds.
	msp := metrics.NewMemStatsAll()
	msp.Start(ctx, 30*time.Second)
	if err := view.Register(msp.DefaultViews()...); err != nil {
		log.Fatalf("Error exporting go memstats view: %v", err)
	}

	// Convert json logging.Config to logging.Config.
	loggingConfig, err := logging.JsonToLoggingConfig(env.LoggingConfigJson)
	if err != nil {
		fmt.Printf("[ERROR] failed to process logging config: %s", err)
		// Use default logging config.
		if loggingConfig, err = logging.NewConfigFromMap(map[string]string{}); err != nil {
			// If this fails, there is no recovering.
			panic(err)
		}
	}
	loggerSugared, _ := logging.NewLoggerFromConfig(loggingConfig, component)
	logger := loggerSugared.Desugar()
	defer flush(loggerSugared)

	// Convert json metrics.ExporterOptions to metrics.ExporterOptions.
	metricsConfig, err := metrics.JsonToMetricsOptions(env.MetricsConfigJson)
	if err != nil {
		logger.Fatal("failed to process metrics options", zap.Error(err))
	}

	if err := metrics.UpdateExporter(*metricsConfig, loggerSugared); err != nil {
		logger.Error("failed to create the metrics exporter", zap.Error(err))
	}

	reporter, err := source.NewStatsReporter()
	if err != nil {
		logger.Error("error building statsreporter", zap.Error(err))
	}

	if err = tracing.SetupStaticPublishing(loggerSugared, "", tracing.OnePercentSampling); err != nil {
		// If tracing doesn't work, we will log an error, but allow the adapter
		// to continue to start.
		logger.Error("Error setting up trace publishing", zap.Error(err))
	}

	eventsClient, err := kncloudevents.NewDefaultClient(env.SinkURI)
	if err != nil {
		logger.Fatal("error building cloud event client", zap.Error(err))
	}

	// Configuring the adapter

	adapter := &cronjobevents.Adapter{
		Schedule:  env.Schedule,
		Data:      env.Data,
		Name:      env.Name,
		Namespace: env.Namespace,
		Reporter:  reporter,
		Client:    eventsClient,
	}

	logger.Info("Starting Receive Adapter", zap.Any("adapter", adapter))

	if err := adapter.Start(ctx, ctx.Done()); err != nil {
		logger.Fatal("Failed to start adapter", zap.Error(err))
	}
}

func flush(logger *zap.SugaredLogger) {
	_ = logger.Sync()
	metrics.FlushExporter()
}
