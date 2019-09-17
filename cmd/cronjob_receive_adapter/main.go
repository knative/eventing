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

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"knative.dev/eventing/pkg/adapter/cronjobevents"
	"knative.dev/eventing/pkg/tracing"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/source"
)

type envConfig struct {
	// Environment variable container schedule.
	Schedule string `envconfig:"SCHEDULE" required:"true"`

	// Environment variable containing data.
	Data string `envconfig:"DATA" required:"true"`

	// Sink for messages.
	Sink string `envconfig:"SINK_URI" required:"true"`

	// Environment variable containing the name of the cron job.
	Name string `envconfig:"NAME" required:"true"`

	// Environment variable containing the namespace of the cron job.
	Namespace string `envconfig:"NAMESPACE" required:"true"`

	// MetricsConfigJson is a json string of metrics.ExporterOptions.
	// This is used to configure the metrics exporter options,
	// the config is stored in a config map inside the controllers
	// namespace and copied here.
	MetricsConfigJson string `envconfig:"K_METRICS_CONFIG" required:"true"`

	// LoggingConfigJson is a json string of logging.Config.
	// This is used to configure the logging config, the config is stored in
	// a config map inside the controllers namespace and copied here.
	LoggingConfigJson string `envconfig:"K_LOGGING_CONFIG" required:"true"`
}

const (
	component = "cronjobsource"
)

func main() {
	flag.Parse()

	ctx := context.Background()
	var env envConfig
	err := envconfig.Process("", &env)
	if err != nil {
		panic(fmt.Sprintf("Error processing env var: %s", err))
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

	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env var", zap.Error(err))
	}
	reporter, err := source.NewStatsReporter()
	if err != nil {
		logger.Error("error building statsreporter", zap.Error(err))
	}
	if err = tracing.SetupStaticPublishing(loggerSugared, "cronjobsource", tracing.OnePercentSampling); err != nil {
		// If tracing doesn't work, we will log an error, but allow the importer to continue to
		// start.
		logger.Error("Error setting up trace publishing", zap.Error(err))
	}

	adapter := &cronjobevents.Adapter{
		Schedule:  env.Schedule,
		Data:      env.Data,
		SinkURI:   env.Sink,
		Name:      env.Name,
		Namespace: env.Namespace,
		Reporter:  reporter,
	}

	logger.Info("Starting Receive Adapter", zap.Any("adapter", adapter))

	stopCh := signals.SetupSignalHandler()

	if err := adapter.Start(ctx, stopCh); err != nil {
		logger.Fatal("Failed to start adapter", zap.Error(err))
	}
}

func flush(logger *zap.SugaredLogger) {
	_ = logger.Sync()
	metrics.FlushExporter()
}
