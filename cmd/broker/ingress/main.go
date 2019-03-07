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
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	"github.com/knative/pkg/signals"
	"go.opencensus.io/exporter/prometheus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.uber.org/zap"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	port        = 8080
	metricsPort = 9090

	readTimeout  = 1 * time.Minute
	writeTimeout = 1 * time.Minute
	wg           sync.WaitGroup
	// brokerName is used to tag metrics.
	brokerName string
)

func main() {
	logConfig := provisioners.NewLoggingConfig()
	logger := provisioners.NewProvisionerLoggerFromConfig(logConfig).Desugar()
	defer logger.Sync()
	flag.Parse()
	logf.SetLogger(logf.ZapLogger(false))

	logger.Info("Starting...")

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		logger.Fatal("Error starting up.", zap.Error(err))
	}

	if err = eventingv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		logger.Fatal("Unable to add eventingv1alpha1 scheme", zap.Error(err))
	}

	c := getRequiredEnv("CHANNEL")

	brokerName, _ = os.LookupEnv("BROKER")

	h := NewHandler(logger, c)

	s := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      h,
		ErrorLog:     zap.NewStdLog(logger),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	err = mgr.Add(&runnableServer{
		logger:          logger,
		s:               s,
		ShutdownTimeout: writeTimeout,
		wg:              &wg,
	})
	if err != nil {
		logger.Fatal("Unable to add runnableServer", zap.Error(err))
	}

	// Metrics
	e, err := prometheus.NewExporter(prometheus.Options{Namespace: metricsNamespace})
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
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	err = mgr.Add(&runnableServer{
		s:               metricsSrv,
		logger:          logger,
		ShutdownTimeout: writeTimeout,
		wg:              &wg,
	})
	if err != nil {
		logger.Fatal("Unable to add metrics runnableServer", zap.Error(err))
	}

	// Set up signals so we handle the first shutdown signal gracefully.
	stopCh := signals.SetupSignalHandler()
	// Start blocks forever.
	if err = mgr.Start(stopCh); err != nil {
		logger.Error("manager.Start() returned an error", zap.Error(err))
	}
	logger.Info("Exiting...")
	wg.Wait()
	logger.Info("Done.")
}

func getRequiredEnv(envKey string) string {
	val, defined := os.LookupEnv(envKey)
	if !defined {
		log.Fatalf("required environment variable not defined '%s'", envKey)
	}
	return val
}

// Handler that takes a single request in and sends it out to a single destination.
type Handler struct {
	receiver    *provisioners.MessageReceiver
	dispatcher  *provisioners.MessageDispatcher
	destination string

	logger *zap.Logger
}

// NewHandler creates a new ingress.Handler.
func NewHandler(logger *zap.Logger, destination string) *Handler {
	handler := &Handler{
		logger:      logger,
		dispatcher:  provisioners.NewMessageDispatcher(logger.Sugar()),
		destination: fmt.Sprintf("http://%s", destination),
	}
	// The receiver function needs to point back at the handler itself, so set it up after
	// initialization.
	handler.receiver = provisioners.NewMessageReceiver(createReceiverFunction(handler), logger.Sugar())

	return handler
}

func createReceiverFunction(f *Handler) func(provisioners.ChannelReference, *provisioners.Message) error {
	return func(_ provisioners.ChannelReference, m *provisioners.Message) error {
		metricsCtx := context.Background()
		if brokerName != "" {
			metricsCtx, _ = tag.New(metricsCtx, tag.Insert(TagBroker, brokerName))
		}

		defer func() {
			stats.Record(metricsCtx, MeasureMessagesTotal.M(1))
		}()

		// TODO Filter. When a message is filtered, add the `filtered` tag to the
		// metrics context.

		metricsCtx, _ = tag.New(metricsCtx, tag.Insert(TagResult, "dispatched"))
		return f.dispatch(m)
	}
}

// http.Handler interface.
func (f *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.receiver.HandleRequest(w, r)
}

// dispatch takes the request, and sends it out the f.destination. If the dispatched
// request returns successfully, then return nil. Else, return an error.
func (f *Handler) dispatch(msg *provisioners.Message) error {
	startTS := time.Now()
	metricsCtx := context.Background()
	if brokerName != "" {
		metricsCtx, _ = tag.New(metricsCtx, tag.Insert(TagBroker, brokerName))
	}
	defer func() {
		dispatchTimeMS := int64(time.Now().Sub(startTS) / time.Millisecond)
		stats.Record(metricsCtx, MeasureDispatchTime.M(dispatchTimeMS))
	}()
	err := f.dispatcher.DispatchMessage(msg, f.destination, "", provisioners.DispatchDefaults{})
	if err != nil {
		f.logger.Error("Error dispatching message", zap.String("destination", f.destination))
		metricsCtx, _ = tag.New(metricsCtx, tag.Insert(TagResult, "error"))
	} else {
		metricsCtx, _ = tag.New(metricsCtx, tag.Insert(TagResult, "success"))
	}
	return err
}

// runnableServer is a small wrapper around http.Server so that it matches the manager.Runnable
// interface.
type runnableServer struct {
	logger *zap.Logger
	s      *http.Server
	// ShutdownTimeout is the duration to wait for the http.Server to gracefully
	// shut down when the stop channel is closed. If this is zero or negative,
	// shutdown will never time out.
	// TODO alternative: zero shuts down immediately, negative means infinite
	ShutdownTimeout time.Duration
	// wg is a temporary workaround for Manager returning immediately without
	// waiting for Runnables to stop. See
	// https://github.com/kubernetes-sigs/controller-runtime/issues/350.
	wg *sync.WaitGroup
}

func (r *runnableServer) Start(stopCh <-chan struct{}) error {
	logger := r.logger.With(zap.String("address", r.s.Addr))
	logger.Info("Listening...")

	errCh := make(chan error)

	go func() {
		r.wg.Add(1)
		err := r.s.ListenAndServe()
		if err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	var err error
	select {
	case err = <-errCh:
		logger.Error("Error running HTTP server", zap.Error(err))
	case <-stopCh:
		var ctx context.Context
		var cancel context.CancelFunc
		if r.ShutdownTimeout > 0 {
			ctx, cancel = context.WithTimeout(context.Background(), r.ShutdownTimeout)
			defer cancel()
		} else {
			ctx = context.Background()
		}
		logger.Info("Shutting down...")
		if err = r.s.Shutdown(ctx); err != nil {
			logger.Error("Shutdown returned an error", zap.Error(err))
		} else {
			logger.Info("Shutdown done")
		}
	}
	r.wg.Done()
	return err
}
