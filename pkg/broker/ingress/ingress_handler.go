package ingress

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	"go.opencensus.io/plugin/ochttp/propagation/b3"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/broker"
	"knative.dev/eventing/pkg/utils"
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

	tctx = addOutGoingTracing(ctx, event, tctx)

	reporterArgs := &ReportArgs{
		ns:          h.Namespace,
		broker:      h.BrokerName,
		eventType:   event.Type(),
		eventSource: event.Source(),
	}

	send := h.decrementTTL(&event)
	if !send {
		// Record the event count.
		h.Reporter.ReportEventCount(reporterArgs, http.StatusBadRequest)
		return nil
	}

	start := time.Now()
	sendingCTX := utils.ContextFrom(tctx, h.ChannelURI)
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

func addOutGoingTracing(ctx context.Context, event cloudevents.Event, tctx cloudevents.HTTPTransportContext) cloudevents.HTTPTransportContext {
	// Inject trace into HTTP header.
	spanContext := trace.FromContext(ctx).SpanContext()
	tctx.Header.Set(b3.TraceIDHeader, spanContext.TraceID.String())
	tctx.Header.Set(b3.SpanIDHeader, spanContext.SpanID.String())
	sampled := 0
	if spanContext.IsSampled() {
		sampled = 1
	}
	tctx.Header.Set(b3.SampledHeader, strconv.Itoa(sampled))

	// Set traceparent, a CloudEvent documented extension attribute for distributed tracing.
	traceParent := strings.Join([]string{"00", spanContext.TraceID.String(), spanContext.SpanID.String(), fmt.Sprintf("%02x", spanContext.TraceOptions)}, "-")
	event.SetExtension(broker.TraceParent, traceParent)
	return tctx
}
