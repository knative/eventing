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
	"net/http"
	"net/url"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	cloudevents "github.com/cloudevents/sdk-go"
	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/broker/ingress"
	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/tracing"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	pkgtracing "knative.dev/pkg/tracing"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	crlog "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

type envConfig struct {
	Broker    string `envconfig:"BROKER" required:"true"`
	Channel   string `envconfig:"CHANNEL" required:"true"`
	Namespace string `envconfig:"NAMESPACE" required:"true"`
}

func main() {
	logConfig := channel.NewLoggingConfig()
	logger := channel.NewProvisionerLoggerFromConfig(logConfig).Desugar()
	defer flush(logger)
	flag.Parse()
	crlog.SetLogger(crlog.ZapLogger(false))

	logger.Info("Starting...")

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		logger.Fatal("Error starting up.", zap.Error(err))
	}

	if err = eventingv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		logger.Fatal("Unable to add eventingv1alpha1 scheme", zap.Error(err))
	}

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logger.Fatal("Failed to process env var", zap.Error(err))
	}

	channelURI := &url.URL{
		Scheme: "http",
		Host:   env.Channel,
		Path:   "/",
	}

	kc := kubernetes.NewForConfigOrDie(mgr.GetConfig())
	configMapWatcher := configmap.NewInformedWatcher(kc, system.Namespace())

	bin := tracing.BrokerIngressName(tracing.BrokerIngressNameArgs{
		Namespace:  env.Namespace,
		BrokerName: env.Broker,
	})
	if err = tracing.SetupDynamicPublishing(logger.Sugar(), configMapWatcher, bin); err != nil {
		logger.Fatal("Error setting up trace publishing", zap.Error(err))
	}

	httpTransport, err := cloudevents.NewHTTPTransport(cloudevents.WithBinaryEncoding(), cehttp.WithMiddleware(pkgtracing.HTTPSpanMiddleware))
	if err != nil {
		logger.Fatal("Unable to create CE transport", zap.Error(err))
	}

	// Liveness check.
	httpTransport.Handler = http.NewServeMux()
	httpTransport.Handler.HandleFunc("/healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})

	ceClient, err := cloudevents.NewClient(httpTransport, cloudevents.WithTimeNow(), cloudevents.WithUUIDs())
	if err != nil {
		logger.Fatal("Unable to create CE client", zap.Error(err))
	}
	h := &ingress.Handler{
		Logger:     logger,
		CeClient:   ceClient,
		ChannelURI: channelURI,
		BrokerName: env.Broker,
	}

	// Run the event handler with the manager.
	err = mgr.Add(h)
	if err != nil {
		logger.Fatal("Unable to add handler", zap.Error(err))
	}

	// TODO watch logging config map.

	// Watch the observability config map and dynamically update metrics exporter.
	configMapWatcher.Watch(metrics.ConfigMapName(), metrics.UpdateExporterFromConfigMap("broker_ingress", logger.Sugar()))

	// Set up signals so we handle the first shutdown signal gracefully.
	stopCh := signals.SetupSignalHandler()

	// configMapWatcher does not block, so start it first.
	if err = configMapWatcher.Start(stopCh); err != nil {
		logger.Warn("Failed to start ConfigMap watcher", zap.Error(err))
	}

	// Start blocks forever.
	if err = mgr.Start(stopCh); err != nil {
		logger.Error("manager.Start() returned an error", zap.Error(err))
	}
	logger.Info("Exiting...")
}

func flush(logger *zap.Logger) {
	logger.Sync()
	metrics.FlushExporter()
}
