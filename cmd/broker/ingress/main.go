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
	"fmt"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"sync"
	"time"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	ceclient "github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/kelseyhightower/envconfig"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/broker"
	"github.com/knative/eventing/pkg/provisioners"
	"github.com/knative/eventing/pkg/utils"
	"github.com/knative/pkg/signals"
	"go.opencensus.io/exporter/prometheus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.uber.org/zap"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	crlog "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	defaultTTL = 255

	metricsPort = 9090

	writeTimeout    = 1 * time.Minute
	shutdownTimeout = 1 * time.Minute

	wg sync.WaitGroup
)

type envConfig struct {
	// Channel where to send the cloudevents.
	Channel string `envconfig:"CHANNEL"`

	// Broker name for this ingress.
	Broker string `envconfig:"BROKER" required:"true"`

	// Namespace of this ingress.
	Namespace string `envconfig:"NAMESPACE" required:"true"`

	// To indicate whether the ingress should allow any event.
	AllowAny bool `envconfig:"ALLOW_ANY" required:"true"`

	// To indicate whether the ingress should auto-register unknown events.
	AutoAdd bool `envconfig:"AUTO_ADD" required:"true"`
}

func main() {
	logConfig := provisioners.NewLoggingConfig()
	logger := provisioners.NewProvisionerLoggerFromConfig(logConfig).Desugar()
	defer logger.Sync()
	flag.Parse()
	crlog.SetLogger(crlog.ZapLogger(false))

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env var", zap.Error(err))
	}

	logger.Info("Starting...")

	namespace := env.Namespace

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{
		Namespace: namespace,
	})
	if err != nil {
		logger.Fatal("Error starting up.", zap.Error(err))
	}

	if err = eventingv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		logger.Fatal("Unable to add eventingv1alpha1 scheme", zap.Error(err))
	}

	brokerName := env.Broker

	channelURI := &url.URL{
		Scheme: "http",
		Host:   env.Channel,
		Path:   "/",
	}

	client := mgr.GetClient()

	policySpec := &eventingv1alpha1.IngressPolicySpec{
		AllowAny: env.AllowAny,
		AutoAdd:  env.AutoAdd,
	}

	ingressPolicy := broker.NewPolicy(logger, client, policySpec, namespace, brokerName, true)

	ceClient, err := ceclient.NewDefault()
	if err != nil {
		logger.Fatal("Unable to create CE client", zap.Error(err))
	}
	h := &handler{
		logger:        logger,
		ceClient:      ceClient,
		channelURI:    channelURI,
		brokerName:    brokerName,
		ingressPolicy: ingressPolicy,
	}

	// Run the event handler with the manager.
	err = mgr.Add(h)
	if err != nil {
		logger.Fatal("Unable to add handler", zap.Error(err))
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

type handler struct {
	logger        *zap.Logger
	ceClient      ceclient.Client
	channelURI    *url.URL
	brokerName    string
	ingressPolicy *broker.IngressPolicy
}

func (h *handler) Start(stopCh <-chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	case <-time.After(shutdownTimeout):
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

	ctx, _ = tag.New(ctx, tag.Insert(TagBroker, h.brokerName))
	defer func() {
		stats.Record(ctx, MeasureMessagesTotal.M(1))
	}()

	send := h.decrementTTL(&event)
	if !send {
		ctx, _ = tag.New(ctx, tag.Insert(TagResult, "droppedDueToTTL"))
		return nil
	}

	if h.allowEvent(ctx, event) {
		ctx, _ = tag.New(ctx, tag.Insert(TagResult, "dispatched"))
		return h.sendEvent(ctx, tctx, event)
	}
	return nil
}

func (h *handler) allowEvent(ctx context.Context, event cloudevents.Event) bool {
	return h.ingressPolicy.AllowEvent(ctx, event)
}

func (h *handler) sendEvent(ctx context.Context, tctx cehttp.TransportContext, event cloudevents.Event) error {
	sendingCTX := broker.SendingContext(ctx, tctx, h.channelURI)

	startTS := time.Now()
	defer func() {
		dispatchTimeMS := int64(time.Now().Sub(startTS) / time.Millisecond)
		stats.Record(sendingCTX, MeasureDispatchTime.M(dispatchTimeMS))
	}()

	_, err := h.ceClient.Send(sendingCTX, event)
	if err != nil {
		sendingCTX, _ = tag.New(sendingCTX, tag.Insert(TagResult, "error"))
	} else {
		sendingCTX, _ = tag.New(sendingCTX, tag.Insert(TagResult, "ok"))
	}
	return err
}

func (h *handler) decrementTTL(event *cloudevents.Event) bool {
	ttl := h.getTTLToSet(event)
	if ttl <= 0 {
		// TODO send to some form of dead letter queue rather than dropping.
		h.logger.Error("Dropping message due to TTL", zap.Any("event", event))
		return false
	}

	event.Context = broker.SetTTL(event.Context, ttl)
	return true
}

func (h *handler) getTTLToSet(event *cloudevents.Event) int {
	ttlInterface, present := event.Context.AsV02().Extensions[broker.V02TTLAttribute]
	if !present {
		h.logger.Debug("No TTL found, defaulting")
		return defaultTTL
	}
	// This should be a JSON number, which json.Unmarshalls as a float64.
	ttl, ok := ttlInterface.(float64)
	if !ok {
		h.logger.Info("TTL attribute wasn't a float64, defaulting", zap.Any("ttlInterface", ttlInterface), zap.Any("typeOf(ttlInterface)", reflect.TypeOf(ttlInterface)))
	}
	return int(ttl) - 1
}
