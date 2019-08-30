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
	"net/http"
	"net/url"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	cloudevents "github.com/cloudevents/sdk-go"
	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"

	"knative.dev/eventing/pkg/broker/ingress"
	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/tracing"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	pkgtracing "knative.dev/pkg/tracing"

	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/clients/kubeclient"
	"knative.dev/pkg/injection/sharedmain"
)

var (
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
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

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logger.Fatal("Failed to process env var", zap.Error(err))
	}

	ctx := signals.NewContext()

	log.Printf("Registering %d clients", len(injection.Default.GetClients()))
	log.Printf("Registering %d informer factories", len(injection.Default.GetInformerFactories()))
	log.Printf("Registering %d informers", len(injection.Default.GetInformers()))

	logger.Info("Starting...")

	cfg, err := sharedmain.GetConfig(*masterURL, *kubeconfig)
	if err != nil {
		log.Fatal("Error building kubeconfig", err)
	}

	ctx, informers := injection.Default.SetupInformers(ctx, cfg)

	channelURI := &url.URL{
		Scheme: "http",
		Host:   env.Channel,
		Path:   "/",
	}

	cmw := configmap.NewInformedWatcher(kubeclient.Get(ctx), system.Namespace())

	// TODO watch logging config map.

	// Watch the observability config map and dynamically update metrics exporter.
	cmw.Watch(metrics.ConfigMapName(), metrics.UpdateExporterFromConfigMap("broker_ingress", logger.Sugar()))

	bin := tracing.BrokerIngressName(tracing.BrokerIngressNameArgs{
		Namespace:  env.Namespace,
		BrokerName: env.Broker,
	})
	if err = tracing.SetupDynamicPublishing(logger.Sugar(), cmw, bin); err != nil {
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
	reporter, err := ingress.NewStatsReporter()
	if err != nil {
		logger.Fatal("Unable to create StatsReporter", zap.Error(err))
	}

	h := &ingress.Handler{
		Logger:     logger,
		CeClient:   ceClient,
		ChannelURI: channelURI,
		BrokerName: env.Broker,
		Namespace:  env.Namespace,
		Reporter:   reporter,
	}

	// configMapWatcher does not block, so start it first.
	if err = cmw.Start(ctx.Done()); err != nil {
		logger.Warn("Failed to start ConfigMap watcher", zap.Error(err))
	}

	// Start all of the informers and wait for them to sync.
	logger.Info("Starting informers.")
	if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
		logger.Fatal("Failed to start informers", zap.Error(err))
	}

	// Start blocks forever.
	if err = h.Start(ctx); err != nil {
		logger.Error("ingress.Start() returned an error", zap.Error(err))
	}
	logger.Info("Exiting...")
}

func flush(logger *zap.Logger) {
	logger.Sync()
	metrics.FlushExporter()
}
