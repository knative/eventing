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
	"errors"
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	ceclient "github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/broker"
	"github.com/knative/eventing/pkg/provisioners"
	"github.com/knative/pkg/signals"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	defaultPort = 8080

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

	channelURI := &url.URL{
		Scheme: "http",
		Host:   getRequiredEnv("CHANNEL"),
		Path:   "/",
	}

	h, err := New(logger, channelURI)
	if err != nil {
		logger.Fatal("Unable to create handler", zap.Error(err))
	}

	err = mgr.Add(h)
	if err != nil {
		logger.Fatal("Unable to add handler", zap.Error(err))
	}

	// Set up signals so we handle the first shutdown signal gracefully.
	stopCh := signals.SetupSignalHandler()
	// Start blocks forever.
	if err = mgr.Start(stopCh); err != nil {
		logger.Error("manager.Start() returned an error", zap.Error(err))
	}
	logger.Info("Exiting...")

	// TODO Gracefully shutdown the server. CloudEvents SDK doesn't seem to let us do that today.
}

func getRequiredEnv(envKey string) string {
	val, defined := os.LookupEnv(envKey)
	if !defined {
		log.Fatalf("required environment variable not defined '%s'", envKey)
	}
	return val
}

func New(logger *zap.Logger, channelURI *url.URL) (*handler, error) {
	ceHttp, err := cehttp.New(cehttp.WithBinaryEncoding(), cehttp.WithPort(defaultPort))
	if err != nil {
		return nil, err
	}
	ceClient, err := ceclient.New(ceHttp)
	if err != nil {
		return nil, err
	}
	return &handler{
		logger:     logger,
		ceClient:   ceClient,
		ceHttp:     ceHttp,
		channelURI: channelURI,
	}, nil
}

type handler struct {
	logger     *zap.Logger
	ceClient   ceclient.Client
	ceHttp     *cehttp.Transport
	channelURI *url.URL
}

func (h *handler) Start(stopCh <-chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer ctx.Done()

	errCh := make(chan error, 1)
	go func() {
		errCh <- h.ceClient.StartReceiver(ctx, h.serveHTTP)
	}()

	// Stop either if the receiver stops (sending to errCh) or if stopCh is closed.
	select {
	case err := <-errCh:
		return err
	case <-stopCh:
		break
	}

	// stopCh has been closed, we need to gracefully shutdown h.ceClient. cancel() will start its
	// shutdown, if it hasn't finished in a reasonable amount of time, just return an error.
	cancel()
	select {
	case err := <-errCh:
		return err
	case <-time.After(writeTimeout):
		return errors.New("timeout shutting down ceClient")
	}
}

func (h *handler) serveHTTP(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
	tctx := cehttp.TransportContextFrom(ctx)
	if tctx.Method != http.MethodPost {
		resp.Status = http.StatusMethodNotAllowed
		return nil
	}

	// tctx.URI is actually the path...
	if tctx.URI != "/" {
		resp.Status = http.StatusNotFound
		return nil
	}

	// TODO Filter.

	return h.sendEvent(ctx, tctx, event)
}

func (h *handler) sendEvent(ctx context.Context, tctx cehttp.TransportContext, event cloudevents.Event) error {
	sendingCTX := broker.SendingContext(ctx, tctx, h.channelURI)
	_, err := h.ceHttp.Send(sendingCTX, event)
	return err
}
