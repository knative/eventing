/*
Copyright 2025 The Knative Authors

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
	"context"
	"log"
	"time"

	"go.uber.org/zap"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	configmap "knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
	k8sruntime "knative.dev/pkg/observability/runtime/k8s"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"

	cmdbroker "knative.dev/eventing/cmd/broker"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/kncloudevents"
	o11yconfigmap "knative.dev/eventing/pkg/observability/configmap"
	"knative.dev/eventing/pkg/observability/otel"
)

const (
	component = "k8s_event_reporter"

	// AggregationWindow is the time window for aggregating events before reporting
	AggregationWindow = 30 * time.Second

	// MaxBufferSize is the maximum number of unique event keys to buffer
	MaxBufferSize = 10000
)

func main() {
	ctx := signals.NewContext()

	cfg := injection.ParseAndGetRESTConfigOrDie()
	ctx = injection.WithConfig(ctx, cfg)

	ctx, informers := injection.Default.SetupInformers(ctx, cfg)

	loggingConfig, err := cmdbroker.GetLoggingConfig(ctx, system.Namespace(), logging.ConfigMapName())
	if err != nil {
		log.Fatal("Error loading/parsing logging configuration:", err)
	}
	sl, atomicLevel := logging.NewLoggerFromConfig(loggingConfig, component)
	logger := sl.Desugar()
	defer flush(sl)

	pprof := k8sruntime.NewProfilingServer(sl.Named("pprof"))

	mp, tp := otel.SetupObservabilityOrDie(ctx, component, sl, pprof)

	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		if err := mp.Shutdown(ctx); err != nil {
			sl.Errorw("Error flushing metrics", zap.Error(err))
		}

		if err := tp.Shutdown(ctx); err != nil {
			sl.Errorw("Error flushing traces", zap.Error(err))
		}
	}()

	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher := configmap.NewInformedWatcher(kubeclient.Get(ctx), system.Namespace())
	// Watch the observability config map and dynamically update metrics exporter.
	configMapWatcher.Watch(o11yconfigmap.Name(), pprof.UpdateFromConfigMap)
	// Watch the observability config map and dynamically update request logs.
	configMapWatcher.Watch(logging.ConfigMapName(), logging.UpdateLevelFromConfigMap(sl, atomicLevel, component))

	logger.Info("Starting the K8s Event Reporter Sink")

	// Create the handler with event aggregator
	aggregator := NewEventAggregator(kubeclient.Get(ctx), sl, AggregationWindow, MaxBufferSize)

	// Decorate contexts with the logger
	ctxFunc := func(ctx context.Context) context.Context {
		return logging.WithLogger(ctx, sl)
	}

	h := &Handler{
		aggregator:  aggregator,
		withContext: ctxFunc,
		logger:      sl,
	}

	handler := otel.NewHandler(h, "receive", mp, tp)

	sm, err := eventingtls.NewServerManager(ctx,
		kncloudevents.NewHTTPEventReceiver(8080),
		nil, // No TLS for now, can be added later
		handler,
		configMapWatcher,
	)
	if err != nil {
		logger.Fatal("failed to start eventingtls server", zap.Error(err))
	}

	// Start the aggregator's reconciliation loop
	go aggregator.Run(ctx)

	// configMapWatcher does not block, so start it first.
	logger.Info("Starting ConfigMap watcher")
	if err = configMapWatcher.Start(ctx.Done()); err != nil {
		logger.Fatal("Failed to start ConfigMap watcher", zap.Error(err))
	}

	// Start informers and wait for them to sync.
	logger.Info("Starting informers.")
	if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
		logger.Fatal("Failed to start informers", zap.Error(err))
	}

	// Start the servers
	logger.Info("Starting...")
	if err = sm.StartServers(ctx); err != nil {
		logger.Fatal("StartServers() returned an error", zap.Error(err))
	}
	logger.Info("Exiting...")
}

func flush(logger *zap.SugaredLogger) {
	_ = logger.Sync()
}
