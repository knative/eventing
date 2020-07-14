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
	"fmt"
	"log"
	"net/http"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/kelseyhightower/envconfig"
	"go.opencensus.io/stats/view"
	"go.uber.org/zap"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/profiling"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/source"

	"knative.dev/eventing/pkg/adapter/v2/util/crstatusevent"
	"knative.dev/eventing/pkg/leaderelection"
)

type Adapter interface {
	Start(ctx context.Context) error
}

type AdapterConstructor func(ctx context.Context, env EnvConfigAccessor, client cloudevents.Client) Adapter

type haEnabledKey struct{}

// WithHAEnabled signals to MainWithContext that it should set up an appropriate leader elector for this component.
func WithHAEnabled(ctx context.Context) context.Context {
	return context.WithValue(ctx, haEnabledKey{}, struct{}{})
}

// IsHAEnabled checks the context for the desire to enable leader elector.
func IsHAEnabled(ctx context.Context) bool {
	val := ctx.Value(haEnabledKey{})
	return val != nil
}

type injectorEnabledKey struct{}

// WithInjectorEnabled signals to MainWithInjectors that it should try to run injectors.
func WithInjectorEnabled(ctx context.Context) context.Context {
	return context.WithValue(ctx, injectorEnabledKey{}, struct{}{})
}

// IsInjectorEnabled checks the context for the desire to enable injectors
func IsInjectorEnabled(ctx context.Context) bool {
	val := ctx.Value(injectorEnabledKey{})
	return val != nil
}

func Main(component string, ector EnvConfigConstructor, ctor AdapterConstructor) {
	ctx := signals.NewContext()

	// TODO(mattmoor): expose a flag that gates this?
	// ctx = WithHAEnabled(ctx)

	MainWithContext(ctx, component, ector, ctor)
}

func MainWithContext(ctx context.Context, component string, ector EnvConfigConstructor, ctor AdapterConstructor) {
	MainWithEnv(ctx, component, ConstructEnvOrDie(ector), ctor)
}

func MainWithEnv(ctx context.Context, component string, env EnvConfigAccessor, ctor AdapterConstructor) {
	if IsInjectorEnabled(ctx) {
		ictx, informers := SetupInformers(ctx, env.GetLogger())
		if informers != nil {
			StartInformers(ctx, informers) // none-blocking
		}
		ctx = ictx
	}
	MainWithInformers(ctx, component, env, ctor)
}

func MainWithInformers(ctx context.Context, component string, env EnvConfigAccessor, ctor AdapterConstructor) {
	if !flag.Parsed() {
		flag.Parse()
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

	var crStatusEventClient *crstatusevent.CRStatusEventClient

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
			crStatusEventClient = crstatusevent.NewCRStatusEventClient(metricsConfig.ConfigMap)
		}
	}

	reporter, err := source.NewStatsReporter()
	if err != nil {
		logger.Error("error building statsreporter", zap.Error(err))
	}

	if err := env.SetupTracing(logger); err != nil {
		// If tracing doesn't work, we will log an error, but allow the adapter
		// to continue to start.
		logger.Error("Error setting up trace publishing", zap.Error(err))
	}

	ceOverrides, err := env.GetCloudEventOverrides()
	if err != nil {
		logger.Error("Error loading cloudevents overrides", zap.Error(err))
	}

	eventsClient, err := NewCloudEventsClientCRStatus(env.GetSink(), ceOverrides, reporter, crStatusEventClient)
	if err != nil {
		logger.Fatal("Error building cloud event client", zap.Error(err))
	}

	// Setup config watcher if enabled.
	if IsConfigMapWatcherEnabled(ctx) {
		cmw := SetupConfigMapWatchOrDie(ctx, component, env.GetNamespace())
		ctx = WithConfigMapWatcher(ctx, cmw)
	}

	// Configuring the adapter
	adapter := ctor(ctx, env, eventsClient)

	// Start config watcher if enabled.
	if IsConfigMapWatcherEnabled(ctx) {
		logger.Info("Starting configuration manager...")
		if err := ConfigMapWatcherFromContext(ctx).Start(ctx.Done()); err != nil {
			logger.Fatalw("Failed to start configuration manager", zap.Error(err))
		}
	}

	// Build the leader elector
	leConfig, err := env.GetLeaderElectionConfig()
	if err != nil {
		logger.Error("Error loading the leader election configuration", zap.Error(err))
	}

	if IsHAEnabled(ctx) {
		// Signal that we are executing in a context with leader election.
		ctx = leaderelection.WithStandardLeaderElectorBuilder(ctx, kubeclient.Get(ctx), *leConfig)
	}

	elector, err := leaderelection.BuildAdapterElector(ctx, adapter)
	if err != nil {
		logger.Fatal("Error creating the adapter elector", zap.Error(err))
	}

	elector.Run(ctx)
}

func ConstructEnvOrDie(ector EnvConfigConstructor) EnvConfigAccessor {
	env := ector()
	if err := envconfig.Process("", env); err != nil {
		log.Fatalf("Error processing env var: %s", err)
	}
	return env
}

func SetupInformers(ctx context.Context, logger *zap.SugaredLogger) (context.Context, []controller.Informer) {
	// Run the injectors, but only if strictly necessary to relax the dependency on kubeconfig.
	if len(injection.Default.GetInformers()) > 0 || len(injection.Default.GetClients()) > 0 ||
		len(injection.Default.GetDucks()) > 0 || len(injection.Default.GetInformerFactories()) > 0 {
		logger.Infof("Registering %d clients", len(injection.Default.GetClients()))
		logger.Infof("Registering %d informer factories", len(injection.Default.GetInformerFactories()))
		logger.Infof("Registering %d informers", len(injection.Default.GetInformers()))
		logger.Infof("Registering %d ducks", len(injection.Default.GetDucks()))

		cfg := sharedmain.ParseAndGetConfigOrDie()
		return injection.Default.SetupInformers(ctx, cfg)
	}
	return ctx, nil
}

func StartInformers(ctx context.Context, informers []controller.Informer) {
	go func() {
		if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
			panic(fmt.Sprintf("Failed to start informers - %s", err))
		}
		<-ctx.Done()
	}()
}

func flush(logger *zap.SugaredLogger) {
	_ = logger.Sync()
	metrics.FlushExporter()
}
