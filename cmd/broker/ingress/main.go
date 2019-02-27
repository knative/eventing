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
	"time"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	"github.com/knative/pkg/signals"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	port = 8080

	readTimeout  = 1 * time.Minute
	writeTimeout = 1 * time.Minute
)

func main() {
	logConfig := provisioners.NewLoggingConfig()
	logger := provisioners.NewProvisionerLoggerFromConfig(logConfig).Desugar()
	defer logger.Sync()
	flag.Parse()

	logger.Info("Starting...")

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		logger.Fatal("Error starting up.", zap.Error(err))
	}

	if err = eventingv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		logger.Fatal("Unable to add eventingv1alpha1 scheme", zap.Error(err))
	}

	c := getRequiredEnv("CHANNEL")

	h := NewHandler(logger, c)

	s := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      h,
		ErrorLog:     zap.NewStdLog(logger),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	err = mgr.Add(&runnableServer{
		logger: logger,
		s:      s,
	})
	if err != nil {
		logger.Fatal("Unable to add runnableServer", zap.Error(err))
	}

	// Set up signals so we handle the first shutdown signal gracefully.
	stopCh := signals.SetupSignalHandler()
	// Start blocks forever.
	if err = mgr.Start(stopCh); err != nil {
		logger.Error("manager.Start() returned an error", zap.Error(err))
	}
	logger.Info("Exiting...")

	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	if err = s.Shutdown(ctx); err != nil {
		logger.Error("Shutdown returned an error", zap.Error(err))
	}
}

func getRequiredEnv(envKey string) string {
	val, defined := os.LookupEnv(envKey)
	if !defined {
		log.Fatalf("required environment variable not defined '%s'", envKey)
	}
	return val
}

// http.Handler that takes a single request in and sends it out to a single destination.
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
		// TODO Filter.
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
	err := f.dispatcher.DispatchMessage(msg, f.destination, "", provisioners.DispatchDefaults{})
	if err != nil {
		f.logger.Error("Error dispatching message", zap.String("destination", f.destination))
	}
	return err
}

// runnableServer is a small wrapper around http.Server so that it matches the manager.Runnable
// interface.
type runnableServer struct {
	logger *zap.Logger
	s      *http.Server
}

func (r *runnableServer) Start(<-chan struct{}) error {
	r.logger.Info("Ingress Listening...", zap.String("Address", r.s.Addr))
	return r.s.ListenAndServe()
}
