package ingress

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/broker"
	"knative.dev/eventing/pkg/utils"
)

var (
	shutdownTimeout = 1 * time.Minute

	defaultTTL int32 = 255
)

type Handler struct {
	Logger     *zap.Logger
	CeClient   cloudevents.Client
	ChannelURI *url.URL
	BrokerName string
	Namespace  string
	Reporter   StatsReporter
}

func (h *Handler) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- h.CeClient.StartReceiver(ctx, h.serveHTTP)
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

func (h *Handler) serveHTTP(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
	// Setting the extension as a string as the CloudEvents sdk does not support non-string extensions.
	event.SetExtension(broker.EventArrivalTime, time.Now().Format(time.RFC3339))
	tctx := cloudevents.HTTPTransportContextFrom(ctx)
	if tctx.Method != http.MethodPost {
		resp.Status = http.StatusMethodNotAllowed
		return nil
	}

	// tctx.URI is actually the path...
	if tctx.URI != "/" {
		resp.Status = http.StatusNotFound
		return nil
	}

	reporterArgs := &ReportArgs{
		ns:        h.Namespace,
		broker:    h.BrokerName,
		eventType: event.Type(),
	}

	send := h.decrementTTL(&event)
	if !send {
		// Record the event count.
		h.Reporter.ReportEventCount(reporterArgs, http.StatusBadRequest)
		return nil
	}

	start := time.Now()
	sendingCTX := utils.ContextFrom(tctx, h.ChannelURI)
	// Due to an issue in utils.ContextFrom, we don't retain the original trace context from ctx, so
	// bring it in manually.
	sendingCTX = trace.NewContext(sendingCTX, trace.FromContext(ctx))

	rctx, _, err := h.CeClient.Send(sendingCTX, event)
	rtctx := cloudevents.HTTPTransportContextFrom(rctx)
	// Record the dispatch time.
	h.Reporter.ReportEventDispatchTime(reporterArgs, rtctx.StatusCode, time.Since(start))
	// Record the event count.
	h.Reporter.ReportEventCount(reporterArgs, rtctx.StatusCode)
	return err
}

func (h *Handler) decrementTTL(event *cloudevents.Event) bool {
	ttl := h.getTTLToSet(event)
	if ttl <= 0 {
		// TODO send to some form of dead letter queue rather than dropping.
		h.Logger.Error("Dropping message due to TTL", zap.Any("event", event))
		return false
	}

	if err := broker.SetTTL(event.Context, ttl); err != nil {
		h.Logger.Error("Failed to set TTL", zap.Error(err))
	}
	return true
}

func (h *Handler) getTTLToSet(event *cloudevents.Event) int32 {
	ttl, err := broker.GetTTL(event.Context)
	if err != nil {
		h.Logger.Debug("Error retrieving TTL, defaulting.", zap.Error(err))
		return defaultTTL
	}
	return ttl - 1
}
