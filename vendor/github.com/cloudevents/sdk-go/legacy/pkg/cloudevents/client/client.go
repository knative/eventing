package client

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudevents/sdk-go/legacy/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/extensions"
	"github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/observability"
	"github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/transport"
	"github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/transport/http"
	"go.opencensus.io/trace"
)

// Client interface defines the runtime contract the CloudEvents client supports.
type Client interface {
	// Send will transmit the given event over the client's configured transport.
	Send(ctx context.Context, event cloudevents.Event) (context.Context, *cloudevents.Event, error)

	// StartReceiver will register the provided function for callback on receipt
	// of a cloudevent. It will also start the underlying transport as it has
	// been configured.
	// This call is blocking.
	// Valid fn signatures are:
	// * func()
	// * func() error
	// * func(context.Context)
	// * func(context.Context) error
	// * func(cloudevents.Event)
	// * func(cloudevents.Event) error
	// * func(context.Context, cloudevents.Event)
	// * func(context.Context, cloudevents.Event) error
	// * func(cloudevents.Event, *cloudevents.EventResponse)
	// * func(cloudevents.Event, *cloudevents.EventResponse) error
	// * func(context.Context, cloudevents.Event, *cloudevents.EventResponse)
	// * func(context.Context, cloudevents.Event, *cloudevents.EventResponse) error
	// Note: if fn returns an error, it is treated as a critical and
	// EventResponse will not be processed.
	StartReceiver(ctx context.Context, fn interface{}) error
}

// New produces a new client with the provided transport object and applied
// client options.
func New(t transport.Transport, opts ...Option) (Client, error) {
	c := &ceClient{
		transport: t,
	}
	if err := c.applyOptions(opts...); err != nil {
		return nil, err
	}
	t.SetReceiver(c)
	return c, nil
}

// NewDefault provides the good defaults for the common case using an HTTP
// Transport client. The http transport has had WithBinaryEncoding http
// transport option applied to it. The client will always send Binary
// encoding but will inspect the outbound event context and match the version.
// The WithTimeNow, WithUUIDs and WithDataContentType("application/json")
// client options are also applied to the client, all outbound events will have
// a time and id set if not already present.
func NewDefault() (Client, error) {
	t, err := http.New(http.WithBinaryEncoding())
	if err != nil {
		return nil, err
	}
	c, err := New(t, WithTimeNow(), WithUUIDs(), WithDataContentType(cloudevents.ApplicationJSON))
	if err != nil {
		return nil, err
	}
	return c, nil
}

type ceClient struct {
	transport transport.Transport
	fn        *receiverFn

	convertFn ConvertFn

	receiverMu        sync.Mutex
	eventDefaulterFns []EventDefaulter

	disableTracePropagation bool
}

// Send transmits the provided event on a preconfigured Transport.
// Send returns a response event if there is a response or an error if there
// was an an issue validating the outbound event or the transport returns an
// error.
func (c *ceClient) Send(ctx context.Context, event cloudevents.Event) (context.Context, *cloudevents.Event, error) {
	ctx, r := observability.NewReporter(ctx, reportSend)

	ctx, span := trace.StartSpan(ctx, clientSpanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	if span.IsRecordingEvents() {
		span.AddAttributes(eventTraceAttributes(event.Context)...)
	}

	rctx, resp, err := c.obsSend(ctx, event)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return rctx, resp, err
}

func (c *ceClient) obsSend(ctx context.Context, event cloudevents.Event) (context.Context, *cloudevents.Event, error) {
	// Confirm we have a transport set.
	if c.transport == nil {
		return ctx, nil, fmt.Errorf("client not ready, transport not initialized")
	}
	// Apply the defaulter chain to the incoming event.
	if len(c.eventDefaulterFns) > 0 {
		for _, fn := range c.eventDefaulterFns {
			event = fn(ctx, event)
		}
	}

	// Set distributed tracing extension.
	if !c.disableTracePropagation {
		if span := trace.FromContext(ctx); span != nil {
			event.Context = event.Context.Clone()
			if err := extensions.FromSpanContext(span.SpanContext()).AddTracingAttributes(event.Context); err != nil {
				return ctx, nil, fmt.Errorf("error setting distributed tracing extension: %w", err)
			}
		}
	}

	// Validate the event conforms to the CloudEvents Spec.
	if err := event.Validate(); err != nil {
		return ctx, nil, err
	}
	// Send the event over the transport.
	return c.transport.Send(ctx, event)
}

// Receive is called from from the transport on event delivery.
func (c *ceClient) Receive(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
	ctx, r := observability.NewReporter(ctx, reportReceive)

	var span *trace.Span
	if !c.transport.HasTracePropagation() {
		if ext, ok := extensions.GetDistributedTracingExtension(event); ok {
			ctx, span = ext.StartChildSpan(ctx, clientSpanName, trace.WithSpanKind(trace.SpanKindServer))
		}
	}
	if span == nil {
		ctx, span = trace.StartSpan(ctx, clientSpanName, trace.WithSpanKind(trace.SpanKindServer))
	}
	defer span.End()
	if span.IsRecordingEvents() {
		span.AddAttributes(eventTraceAttributes(event.Context)...)
	}

	err := c.obsReceive(ctx, event, resp)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return err
}

func (c *ceClient) obsReceive(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
	if c.fn != nil {
		err := c.fn.invoke(ctx, event, resp)

		// Apply the defaulter chain to the outgoing event.
		if err == nil && resp != nil && resp.Event != nil && len(c.eventDefaulterFns) > 0 {
			for _, fn := range c.eventDefaulterFns {
				*resp.Event = fn(ctx, *resp.Event)
			}
			// Validate the event conforms to the CloudEvents Spec.
			if err := resp.Event.Validate(); err != nil {
				return fmt.Errorf("cloudevent validation failed on response event: %v", err)
			}
		}
		return err
	}
	return nil
}

// StartReceiver sets up the given fn to handle Receive.
// See Client.StartReceiver for details. This is a blocking call.
func (c *ceClient) StartReceiver(ctx context.Context, fn interface{}) error {
	c.receiverMu.Lock()
	defer c.receiverMu.Unlock()

	if c.transport == nil {
		return fmt.Errorf("client not ready, transport not initialized")
	}
	if c.fn != nil {
		return fmt.Errorf("client already has a receiver")
	}

	if fn, err := receiver(fn); err != nil {
		return err
	} else {
		c.fn = fn
	}

	defer func() {
		c.fn = nil
	}()

	return c.transport.StartReceiver(ctx)
}

func (c *ceClient) applyOptions(opts ...Option) error {
	for _, fn := range opts {
		if err := fn(c); err != nil {
			return err
		}
	}
	return nil
}

// Convert implements transport Converter.Convert.
func (c *ceClient) Convert(ctx context.Context, m transport.Message, err error) (*cloudevents.Event, error) {
	if c.convertFn != nil {
		return c.convertFn(ctx, m, err)
	}
	return nil, err
}
