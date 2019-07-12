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
	"log"

	"github.com/kelseyhightower/envconfig"
	"github.com/knative/eventing/pkg/adapter/cronjobevents"
	"github.com/knative/eventing/pkg/tracing"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/context"
	"knative.dev/pkg/signals"
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
}

func main() {
	flag.Parse()

	ctx := context.Background()
	logCfg := zap.NewProductionConfig()
	logCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := logCfg.Build()
	if err != nil {
		log.Fatalf("Unable to create logger: %v", err)
	}

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env var", zap.Error(err))
	}

	if err = tracing.SetupStaticZipkinPublishing("cronjobsource", tracing.OnePercentSampling); err != nil {
		// If tracing doesn't work, we will log an error, but allow the importer to continue to
		// start.
		logger.Error("Error setting up Zipkin publishing", zap.Error(err))
	}

	adapter := &cronjobevents.Adapter{
		Schedule:  env.Schedule,
		Data:      env.Data,
		SinkURI:   env.Sink,
		Name:      env.Name,
		Namespace: env.Namespace,
	}

	logger.Info("Starting Receive Adapter", zap.Any("adapter", adapter))

	stopCh := signals.SetupSignalHandler()

	if err := adapter.Start(ctx, stopCh); err != nil {
		logger.Fatal("Failed to start adapter", zap.Error(err))
	}
}
