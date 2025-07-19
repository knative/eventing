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

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/metrics/source"
	"knative.dev/eventing/pkg/observability"
	"knative.dev/eventing/pkg/observability/otel"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	k8sruntime "knative.dev/pkg/observability/runtime/k8s"
	"knative.dev/pkg/signals"
)

type MessageAdapter interface {
	Start(ctx context.Context) error
}

type MessageAdapterConstructor func(ctx context.Context, env EnvConfigAccessor, sink duckv1.Addressable, reporter source.StatsReporter) MessageAdapter

func MainMessageAdapter(component string, ector EnvConfigConstructor, ctor MessageAdapterConstructor) {
	MainMessageAdapterWithContext(signals.NewContext(), component, ector, ctor)
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

	obsCfg, err := env.GetObservabilityConfig()
	if err != nil {
		logger.Fatalw("Error parsing observability config from env", zap.Error(err))
	}
	obsCfg = observability.MergeWithDefaults(obsCfg) // ensure that we get defaults if the var is not set

	ctx = observability.WithConfig(ctx, obsCfg)

	pprof := k8sruntime.NewProfilingServer(logger.Named("pprof"))

	_, _ = otel.SetupObservabilityOrDie(ctx, component, logger, pprof)

	reporter, err := source.NewStatsReporter()
	if err != nil {
		logger.Error("error building statsreporter", zap.Error(err))
	}

	sinkURL, err := apis.ParseURL(env.GetSink())
	if err != nil {
		logger.Fatal("error parsing sink URL", zap.Error(err))
	}
	sink := duckv1.Addressable{
		URL:     sinkURL,
		CACerts: env.GetCACerts(),
	}

	// Configuring the adapter
	adapter := ctor(ctx, env, sink, reporter)

	// Finally start the adapter (blocking)
	if err := adapter.Start(ctx); err != nil {
		logging.FromContext(ctx).Warn("Start returned an error", zap.Error(err))
	}
}
