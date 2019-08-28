/*
 * Copyright 2019 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"flag"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/clients/kubeclient"
	"knative.dev/pkg/injection/sharedmain"
	"log"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"knative.dev/eventing/pkg/broker/filter"
	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/tracing"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"

	"knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/trigger"
)

var (
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
)

type envConfig struct {
	Broker    string `envconfig:"BROKER" required:"true"`
	Namespace string `envconfig:"NAMESPACE" required:"true"`
}

func main() {
	logConfig := channel.NewLoggingConfig()
	logConfig.LoggingLevel["provisioner"] = zapcore.DebugLevel
	logger := channel.NewProvisionerLoggerFromConfig(logConfig).Desugar()
	flag.Parse()

	defer flush(logger)

	ctx := signals.NewContext()

	log.Printf("Registering %d clients", len(injection.Default.GetClients()))
	log.Printf("Registering %d informer factories", len(injection.Default.GetInformerFactories()))
	log.Printf("Registering %d informers", len(injection.Default.GetInformers()))

	logger.Info("Starting...")

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logger.Fatal("Failed to process env var", zap.Error(err))
	}

	cfg, err := sharedmain.GetConfig(*masterURL, *kubeconfig)
	if err != nil {
		log.Fatal("Error building kubeconfig", err)
	}

	configMapWatcher := configmap.NewInformedWatcher(kubeclient.Get(ctx), system.Namespace())

	bin := tracing.BrokerFilterName(tracing.BrokerFilterNameArgs{
		Namespace:  env.Namespace,
		BrokerName: env.Broker,
	})
	if err = tracing.SetupDynamicPublishing(logger.Sugar(), configMapWatcher, bin); err != nil {
		logger.Fatal("Error setting up trace publishing", zap.Error(err))
	}

	reporter, err := filter.NewStatsReporter()
	if err != nil {
		logger.Fatal("Error creating stats reporter", zap.Error(err))
	}

	ctx, informers := injection.Default.SetupInformers(ctx, cfg)
	// Start all of the informers and wait for them to sync.
	logger.Info("Starting informers.")
	if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
		logger.Fatal("Failed to start informers", zap.Error(err))
	}

	triggerInformer := trigger.Get(ctx)

	// We are running both the receiver (takes messages in from the Broker) and the dispatcher (send
	// the messages to the triggers' subscribers) in this binary.
	handler, err := filter.NewHandler(logger, triggerInformer.Lister(), reporter)
	if err != nil {
		logger.Fatal("Error creating Handler", zap.Error(err))
	}

	// TODO watch logging config map.

	// Watch the observability config map and dynamically update metrics exporter.
	configMapWatcher.Watch(metrics.ConfigMapName(), metrics.UpdateExporterFromConfigMap("broker_filter", logger.Sugar()))

	// Set up signals so we handle the first shutdown signal gracefully.
	stopCh := signals.SetupSignalHandler()

	// configMapWatcher does not block, so start it first.
	if err = configMapWatcher.Start(stopCh); err != nil {
		logger.Warn("Failed to start ConfigMap watcher", zap.Error(err))
	}

	// Start blocks forever.
	logger.Info("Filter starting...")

	err = handler.Start(ctx)
	if err != nil {
		logger.Fatal("handler.Start() returned an error", zap.Error(err))
	}
	logger.Info("Exiting...")
}

func flush(logger *zap.Logger) {
	logger.Sync()
	metrics.FlushExporter()
}
