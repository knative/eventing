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
	"log"

	"k8s.io/client-go/kubernetes"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"

	"knative.dev/eventing/pkg/broker/filter"
	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/tracing"

	"knative.dev/pkg/injection/sharedmain"

	eventingv1alpha1 "knative.dev/eventing/pkg/client/clientset/versioned"
	eventinginformers "knative.dev/eventing/pkg/client/informers/externalversions"
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

	logger.Info("Starting...")

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logger.Fatal("Failed to process env var", zap.Error(err))
	}

	cfg, err := sharedmain.GetConfig(*masterURL, *kubeconfig)
	if err != nil {
		log.Fatal("Error building kubeconfig", err)
	}

	kubeClient := kubernetes.NewForConfigOrDie(cfg)

	eventingClient := eventingv1alpha1.NewForConfigOrDie(cfg)
	eventingFactory := eventinginformers.NewSharedInformerFactoryWithOptions(eventingClient,
		controller.GetResyncPeriod(ctx),
		eventinginformers.WithNamespace(env.Namespace))
	triggerInformer := eventingFactory.Eventing().V1alpha1().Triggers()

	cmw := configmap.NewInformedWatcher(kubeClient, system.Namespace())

	bin := tracing.BrokerFilterName(tracing.BrokerFilterNameArgs{
		Namespace:  env.Namespace,
		BrokerName: env.Broker,
	})
	if err = tracing.SetupDynamicPublishing(logger.Sugar(), cmw, bin); err != nil {
		logger.Fatal("Error setting up trace publishing", zap.Error(err))
	}

	reporter, err := filter.NewStatsReporter()
	if err != nil {
		logger.Fatal("Error creating stats reporter", zap.Error(err))
	}

	// We are running both the receiver (takes messages in from the Broker) and the dispatcher (send
	// the messages to the triggers' subscribers) in this binary.
	handler, err := filter.NewHandler(logger, triggerInformer.Lister().Triggers(env.Namespace), reporter)
	if err != nil {
		logger.Fatal("Error creating Handler", zap.Error(err))
	}

	// TODO watch logging config map.

	// Watch the observability config map and dynamically update metrics exporter.
	cmw.Watch(metrics.ConfigMapName(), metrics.UpdateExporterFromConfigMap("broker_filter", logger.Sugar()))

	// configMapWatcher does not block, so start it first.
	if err = cmw.Start(ctx.Done()); err != nil {
		logger.Warn("Failed to start ConfigMap watcher", zap.Error(err))
	}

	// Start all of the informers and wait for them to sync.
	logger.Info("Starting informer.")

	go eventingFactory.Start(ctx.Done())
	eventingFactory.WaitForCacheSync(ctx.Done())

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
