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
	"time"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/kelseyhightower/envconfig"
	"go.opencensus.io/stats/view"
	"go.uber.org/zap"

	cmdbroker "knative.dev/eventing/cmd/broker"
	"knative.dev/eventing/pkg/broker"
	"knative.dev/eventing/pkg/broker/ingress"
	"knative.dev/eventing/pkg/kncloudevents"
	cmpresources "knative.dev/eventing/pkg/reconciler/configmappropagation/resources"
	namespaceresources "knative.dev/eventing/pkg/reconciler/namespace/resources"
	"knative.dev/eventing/pkg/tracing"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/signals"
	pkgtracing "knative.dev/pkg/tracing"
	tracingconfig "knative.dev/pkg/tracing/config"
)

var (
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
)

// TODO make these constants configurable (either as env variables, config map, or part of broker spec).
//  Issue: https://github.com/knative/eventing/issues/1777
const (
	// Constants for the underlying HTTP Client transport. These would enable better connection reuse.
	// Purposely set them to be equal, as the ingress only connects to its channel.
	// These are magic numbers, partly set based on empirical evidence running performance workloads, and partly
	// based on what serving is doing. See https://github.com/knative/serving/blob/master/pkg/network/transports.go.
	defaultMaxIdleConnections              = 1000
	defaultMaxIdleConnectionsPerHost       = 1000
	defaultTTL                       int32 = 255
	defaultMetricsPort                     = 9092
	component                              = "broker_ingress"
)

type envConfig struct {
	Broker        string `envconfig:"BROKER" required:"true"`
	Channel       string `envconfig:"CHANNEL" required:"true"`
	Namespace     string `envconfig:"NAMESPACE" required:"true"`
	PodName       string `split_words:"true" required:"true"`
	ContainerName string `split_words:"true" required:"true"`
}

func main() {
	flag.Parse()

	ctx := signals.NewContext()

	// Report stats on Go memory usage every 30 seconds.
	msp := metrics.NewMemStatsAll()
	msp.Start(ctx, 30*time.Second)
	if err := view.Register(msp.DefaultViews()...); err != nil {
		log.Fatalf("Error exporting go memstats view: %v", err)
	}

	cfg, err := sharedmain.GetConfig(*masterURL, *kubeconfig)
	if err != nil {
		log.Fatal("Error building kubeconfig", err)
	}

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env var", zap.Error(err))
	}

	log.Printf("Registering %d clients", len(injection.Default.GetClients()))
	log.Printf("Registering %d informer factories", len(injection.Default.GetInformerFactories()))
	log.Printf("Registering %d informers", len(injection.Default.GetInformers()))

	ctx, informers := injection.Default.SetupInformers(ctx, cfg)

	loggingConfigMapName := cmpresources.MakeCopyConfigMapName(namespaceresources.DefaultConfigMapPropagationName, logging.ConfigMapName())
	metricsConfigMapName := cmpresources.MakeCopyConfigMapName(namespaceresources.DefaultConfigMapPropagationName, metrics.ConfigMapName())

	loggingConfig, err := cmdbroker.GetLoggingConfig(ctx, env.Namespace, loggingConfigMapName)
	if err != nil {
		log.Fatal("Error loading/parsing logging configuration:", err)
	}
	sl, atomicLevel := logging.NewLoggerFromConfig(loggingConfig, component)
	logger := sl.Desugar()
	defer flush(sl)

	logger.Info("Starting the Broker Ingress")

	channelURI := &url.URL{
		Scheme: "http",
		Host:   env.Channel,
		Path:   "/",
	}

	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher := configmap.NewInformedWatcher(kubeclient.Get(ctx), env.Namespace)
	// Watch the observability config map and dynamically update metrics exporter.
	updateFunc, err := metrics.UpdateExporterFromConfigMapWithOpts(metrics.ExporterOptions{
		Component:      component,
		PrometheusPort: defaultMetricsPort,
	}, sl)
	if err != nil {
		logger.Fatal("Failed to create metrics exporter update function", zap.Error(err))
	}
	configMapWatcher.Watch(metricsConfigMapName, updateFunc)
	// TODO change the component name to broker once Stackdriver metrics are approved.
	// Watch the observability config map and dynamically update request logs.
	configMapWatcher.Watch(loggingConfigMapName, logging.UpdateLevelFromConfigMap(sl, atomicLevel, component))

	bin := tracing.BrokerIngressName(tracing.BrokerIngressNameArgs{
		Namespace:  env.Namespace,
		BrokerName: env.Broker,
	})
	if err = tracing.SetupDynamicPublishing(sl, configMapWatcher, bin,
		cmpresources.MakeCopyConfigMapName(namespaceresources.DefaultConfigMapPropagationName, tracingconfig.ConfigName)); err != nil {
		logger.Fatal("Error setting up trace publishing", zap.Error(err))
	}

	httpTransport, err := cloudevents.NewHTTPTransport(cloudevents.WithBinaryEncoding(), cloudevents.WithMiddleware(pkgtracing.HTTPSpanMiddleware))
	if err != nil {
		logger.Fatal("Unable to create CE transport", zap.Error(err))
	}

	// Liveness check.
	httpTransport.Handler = http.NewServeMux()
	httpTransport.Handler.HandleFunc("/healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})

	connectionArgs := kncloudevents.ConnectionArgs{
		MaxIdleConns:        defaultMaxIdleConnections,
		MaxIdleConnsPerHost: defaultMaxIdleConnectionsPerHost,
	}
	ceClient, err := kncloudevents.NewDefaultClientGivenHttpTransport(
		httpTransport,
		&connectionArgs)
	if err != nil {
		logger.Fatal("Unable to create CE client", zap.Error(err))
	}

	reporter := ingress.NewStatsReporter(env.PodName, env.ContainerName)

	h := &ingress.Handler{
		Logger:     logger,
		CeClient:   ceClient,
		ChannelURI: channelURI,
		BrokerName: env.Broker,
		Namespace:  env.Namespace,
		Reporter:   reporter,
		Defaulter:  broker.TTLDefaulter(logger, defaultTTL),
	}

	// configMapWatcher does not block, so start it first.
	if err = configMapWatcher.Start(ctx.Done()); err != nil {
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

func flush(logger *zap.SugaredLogger) {
	_ = logger.Sync()
	metrics.FlushExporter()
}
