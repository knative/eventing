package ingress

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"time"

	"github.com/cloudevents/sdk-go"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/broker"
)

var (
	shutdownTimeout = 1 * time.Minute

	defaultTTL = 255
)

type Handler struct {
	Logger     *zap.Logger
	CeClient   cloudevents.Client
	ChannelURI *url.URL
	BrokerName string
	Namespace  string
	Reporter   StatsReporter
}

func (h *Handler) Start(stopCh <-chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- h.CeClient.StartReceiver(ctx, h.serveHTTP)
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

func (h *Handler) serveHTTP(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
	event.SetExtension(broker.EventArrivalTime, time.Now())
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
		h.Reporter.ReportEventCount(reporterArgs, errors.New("dropped due to TTL"))
		return nil
	}

	start := time.Now()
	sendingCTX := broker.SendingContext(ctx, tctx, h.ChannelURI)
	// TODO get HTTP status codes and use those.
	_, err := h.CeClient.Send(sendingCTX, event)
	// Record the dispatch time.
	h.Reporter.ReportDispatchTime(reporterArgs, err, time.Since(start))
	// Record the event count.
	h.Reporter.ReportEventCount(reporterArgs, err)
	return err
}

func (h *Handler) decrementTTL(event *cloudevents.Event) bool {
	ttl := h.getTTLToSet(event)
	if ttl <= 0 {
		// TODO send to some form of dead letter queue rather than dropping.
		h.Logger.Error("Dropping message due to TTL", zap.Any("event", event))
		return false
	}

	var err error
	event.Context, err = broker.SetTTL(event.Context, ttl)
	if err != nil {
		h.Logger.Error("failed to set TTL", zap.Error(err))
	}
	return true
}

func (h *Handler) getTTLToSet(event *cloudevents.Event) int {
	ttlInterface, _ := broker.GetTTL(event.Context)
	if ttlInterface == nil {
		h.Logger.Debug("No TTL found, defaulting")
		return defaultTTL
	}
	// This should be a JSON number, which json.Unmarshalls as a float64.
	ttl, ok := ttlInterface.(float64)
	if !ok {
		h.Logger.Info("TTL attribute wasn't a float64, defaulting", zap.Any("ttlInterface", ttlInterface), zap.Any("typeOf(ttlInterface)", reflect.TypeOf(ttlInterface)))
	}
	return int(ttl) - 1
}
