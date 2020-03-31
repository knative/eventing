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

package ingress

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v1"
	"github.com/cloudevents/sdk-go/v1/cloudevents/client"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/broker"
	"knative.dev/eventing/pkg/utils"
)

var (
	shutdownTimeout = 1 * time.Minute
)

type Handler struct {
	Logger   *zap.Logger
	CeClient cloudevents.Client
	Reporter StatsReporter

	Defaulter client.EventDefaulter
}

func (h *Handler) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- h.CeClient.StartReceiver(ctx, h.receive)
	}()

	// Stop either if the receiver stops (sending to errCh) or if stopCh is closed.
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
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

func (h *Handler) receive(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
	// Setting the extension as a string as the CloudEvents sdk does not support non-string extensions.
	event.SetExtension(broker.EventArrivalTime, cloudevents.Timestamp{Time: time.Now()})
	tctx := cloudevents.HTTPTransportContextFrom(ctx)
	if tctx.Method != http.MethodPost {
		resp.Status = http.StatusMethodNotAllowed
		return nil
	}

	// tctx.URI is actually the request uri...
	if tctx.URI == "/" {
		resp.Status = http.StatusNotFound
		return nil
	}
	pieces := strings.Split(tctx.URI, "/")
	if len(pieces) != 3 {
		h.Logger.Info("Malformed uri", zap.String("URI", tctx.URI))
		resp.Status = http.StatusNotFound
		return nil
	}
	brokerNamespace := pieces[1]
	brokerName := pieces[2]

	reporterArgs := &ReportArgs{
		ns:        brokerNamespace,
		broker:    brokerName,
		eventType: event.Type(),
	}

	if h.Defaulter != nil {
		event = h.Defaulter(ctx, event)
	}

	if ttl, err := broker.GetTTL(event.Context); err != nil || ttl <= 0 {
		h.Logger.Debug("dropping event based on TTL status.",
			zap.Int32("TTL", ttl),
			zap.String("event.id", event.ID()),
			zap.Error(err))
		// Record the event count.
		h.Reporter.ReportEventCount(reporterArgs, http.StatusBadRequest)
		return nil
	}

	start := time.Now()
	// TODO: Today these are pre-deterministic, change this watch for
	// channels and look up from the channels Status
	channelURI := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s-kne-trigger-kn-channel.%s.svc.%s", brokerName, brokerNamespace, utils.GetClusterDomainName()),
		Path:   "/",
	}
	sendingCTX := utils.SendingContextFrom(ctx, tctx, channelURI)

	rctx, _, err := h.CeClient.Send(sendingCTX, event)
	rtctx := cloudevents.HTTPTransportContextFrom(rctx)
	// Record the dispatch time.
	h.Reporter.ReportEventDispatchTime(reporterArgs, rtctx.StatusCode, time.Since(start))
	// Record the event count.
	h.Reporter.ReportEventCount(reporterArgs, rtctx.StatusCode)
	return err
}
