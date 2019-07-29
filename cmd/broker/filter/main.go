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
	"sync"
	"time"

	"contrib.go.opencensus.io/exporter/prometheus"
	"github.com/kelseyhightower/envconfig"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/broker"
	"github.com/knative/eventing/pkg/provisioners"
	"github.com/knative/eventing/pkg/tracing"
	"github.com/knative/eventing/pkg/utils"
	"go.opencensus.io/stats/view"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type envConfig struct {
	Broker    string `envconfig:"BROKER" required:"true"`
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
	logConfig.LoggingLevel["provisioner"] = zapcore.DebugLevel
	logger := provisioners.NewProvisionerLoggerFromConfig(logConfig).Desugar()
	defer logger.Sync()

	flag.Parse()

	logger.Info("Starting...")

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logger.Fatal("Failed to process env var", zap.Error(err))
	}

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{
		Namespace: env.Namespace,
	})
	if err != nil {
		logger.Fatal("Error starting up.", zap.Error(err))
	}

	if err = eventingv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		logger.Fatal("Unable to add eventingv1alpha1 scheme", zap.Error(err))
	}

	kc := kubernetes.NewForConfigOrDie(mgr.GetConfig())
	configMapWatcher := configmap.NewInformedWatcher(kc, system.Namespace())

	zipkinServiceName := tracing.BrokerFilterName(tracing.BrokerFilterNameArgs{
		Namespace:  env.Namespace,
		BrokerName: env.Broker,
	})
	if err = tracing.SetupDynamicZipkinPublishing(logger.Sugar(), configMapWatcher, zipkinServiceName); err != nil {
		logger.Fatal("Error setting up Zipkin publishing", zap.Error(err))
	}

	// We are running both the receiver (takes messages in from the Broker) and the dispatcher (send
	// the messages to the triggers' subscribers) in this binary.
	receiver, err := broker.New(logger, mgr.GetClient())
	if err != nil {
		logger.Fatal("Error creating Receiver", zap.Error(err))
	}
	err = mgr.Add(receiver)
	if err != nil {
		logger.Fatal("Unable to start the receiver", zap.Error(err), zap.Any("receiver", receiver))
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
	logger.Info("Manager starting...")
	err = mgr.Start(stopCh)
	if err != nil {
		logger.Fatal("Manager.Start() returned an error", zap.Error(err))
	}
	logger.Info("Exiting...")

	go func() {
		<-time.After(shutdownTimeout)
		log.Fatalf("Shutdown took longer than %v", shutdownTimeout)
	}()

	// Wait for runnables to stop. This blocks indefinitely, but the above
	// goroutine will exit the process if it takes longer than shutdownTimeout.
	wg.Wait()
	logger.Info("Done.")
}
