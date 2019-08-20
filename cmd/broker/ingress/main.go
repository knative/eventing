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
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"contrib.go.opencensus.io/exporter/prometheus"
	cloudevents "github.com/cloudevents/sdk-go"
	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/kelseyhightower/envconfig"
	"go.opencensus.io/stats/view"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/broker/ingress"
	"knative.dev/eventing/pkg/provisioners"
	"knative.dev/eventing/pkg/tracing"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/configmap"
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

var (
	metricsPort = 9090

	writeTimeout    = 1 * time.Minute
	shutdownTimeout = 1 * time.Minute

	wg sync.WaitGroup
)

func main() {
	logConfig := provisioners.NewLoggingConfig()
	logger := provisioners.NewProvisionerLoggerFromConfig(logConfig).Desugar()
	defer logger.Sync()
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

	zipkinServiceName := tracing.BrokerIngressName(tracing.BrokerIngressNameArgs{
		Namespace:  env.Namespace,
		BrokerName: env.Broker,
	})
	if err = tracing.SetupDynamicZipkinPublishing(logger.Sugar(), configMapWatcher, zipkinServiceName); err != nil {
		logger.Fatal("Error setting up Zipkin publishing", zap.Error(err))
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

	// Metrics
	e, err := prometheus.NewExporter(prometheus.Options{})
	if err != nil {
		logger.Fatal("Unable to create Prometheus exporter", zap.Error(err))
	}
	view.RegisterExporter(e)
	sm := http.NewServeMux()
	sm.Handle("/metrics", e)
	metricsSrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", metricsPort),
		Handler:      e,
		ErrorLog:     zap.NewStdLog(logger),
		WriteTimeout: writeTimeout,
	}

	err = mgr.Add(&utils.RunnableServer{
		Server:          metricsSrv,
		ShutdownTimeout: shutdownTimeout,
		WaitGroup:       &wg,
	})
	if err != nil {
		logger.Fatal("Unable to add metrics runnableServer", zap.Error(err))
	}

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

	// TODO Gracefully shutdown the ingress server. CloudEvents SDK doesn't seem
	// to let us do that today.
	go func() {
		<-time.After(shutdownTimeout)
		log.Fatalf("Shutdown took longer than %v", shutdownTimeout)
	}()

	// Wait for runnables to stop. This blocks indefinitely, but the above
	// goroutine will exit the process if it takes longer than shutdownTimeout.
	wg.Wait()
	logger.Info("Done.")
}
