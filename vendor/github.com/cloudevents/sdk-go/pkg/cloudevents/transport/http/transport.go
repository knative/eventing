package http

import (
	"bytes"
	"context"
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/observability"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	cecontext "github.com/cloudevents/sdk-go/pkg/cloudevents/context"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport"
)

type EncodingSelector func(e cloudevents.Event) Encoding

// Transport adheres to transport.Transport.
var _ transport.Transport = (*Transport)(nil)

const (
	// DefaultShutdownTimeout defines the default timeout given to the http.Server when calling Shutdown.
	DefaultShutdownTimeout = time.Minute * 1
)

// Transport acts as both a http client and a http handler.
type Transport struct {
	// The encoding used to select the codec for outbound events.
	Encoding Encoding
	// DefaultEncodingSelectionFn allows for other encoding selection strategies to be injected.
	DefaultEncodingSelectionFn EncodingSelector

	// ShutdownTimeout defines the timeout given to the http.Server when calling Shutdown.
	// If nil, DefaultShutdownTimeout is used.
	ShutdownTimeout *time.Duration

	// Sending

	// Client is the http client that will be used to send requests.
	// If nil, the Transport will create a one.
	Client *http.Client
	// Req is the base http request that is used for http.Do.
	// Only .Method, .URL, and .Header is considered.
	// If not set, Req.Method defaults to POST.
	// Req.URL or context.WithTarget(url) are required for sending.
	Req *http.Request

	// Receiving

	// Receiver is invoked target for incoming events.
	Receiver transport.Receiver
	// Port is the port to bind the receiver to. Defaults to 8080.
	Port *int
	// Path is the path to bind the receiver to. Defaults to "/".
	Path string
	// Handler is the handler the http Server will use. Use this to reuse the
	// http server. If nil, the Transport will create a one.
	Handler *http.ServeMux

	realPort          int
	server            *http.Server
	handlerRegistered bool
	codec             transport.Codec
	// Create Mutex
	crMu sync.Mutex
	// Receive Mutex
	reMu sync.Mutex
}

func New(opts ...Option) (*Transport, error) {
	t := &Transport{
		Req: &http.Request{
			Method: http.MethodPost,
		},
	}
	if err := t.applyOptions(opts...); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Transport) applyOptions(opts ...Option) error {
	for _, fn := range opts {
		if err := fn(t); err != nil {
			return err
		}
	}
	return nil
}

func (t *Transport) loadCodec() bool {
	if t.codec == nil {
		t.crMu.Lock()
		if t.DefaultEncodingSelectionFn != nil && t.Encoding != Default {
			log.Printf("[warn] Transport has a DefaultEncodingSelectionFn set but Encoding is not Default. DefaultEncodingSelectionFn will be ignored.")
		}
		t.codec = &Codec{
			Encoding:                   t.Encoding,
			DefaultEncodingSelectionFn: t.DefaultEncodingSelectionFn,
		}
		t.crMu.Unlock()
	}
	return true
}

func copyHeaders(from, to http.Header) {
	if from == nil || to == nil {
		return
	}
	for header, values := range from {
		for _, value := range values {
			to.Add(header, value)
		}
	}
}

// Send implements Transport.Send
func (t *Transport) Send(ctx context.Context, event cloudevents.Event) (*cloudevents.Event, error) {
	ctx, r := observability.NewReporter(ctx, reportSend)
	resp, err := t.obsSend(ctx, event)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return resp, err
}

func (t *Transport) obsSend(ctx context.Context, event cloudevents.Event) (*cloudevents.Event, error) {
	if t.Client == nil {
		t.crMu.Lock()
		t.Client = &http.Client{}
		t.crMu.Unlock()
	}

	req := http.Request{
		Header: HeaderFrom(ctx),
	}
	if t.Req != nil {
		req.Method = t.Req.Method
		req.URL = t.Req.URL
		copyHeaders(t.Req.Header, req.Header)
	}

	// Override the default request with target from context.
	if target := cecontext.TargetFrom(ctx); target != nil {
		req.URL = target
	}

	if ok := t.loadCodec(); !ok {
		return nil, fmt.Errorf("unknown encoding set on transport: %d", t.Encoding)
	}

	msg, err := t.codec.Encode(event)
	if err != nil {
		return nil, err
	}

	if m, ok := msg.(*Message); ok {
		copyHeaders(m.Header, req.Header)

		req.Body = ioutil.NopCloser(bytes.NewBuffer(m.Body))
		req.ContentLength = int64(len(m.Body))
		req.Close = true

		return httpDo(ctx, t.Client, &req, func(resp *http.Response, err error) (*cloudevents.Event, error) {
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			body, _ := ioutil.ReadAll(resp.Body)
			msg := &Message{
				Header: resp.Header,
				Body:   body,
			}

			var respEvent *cloudevents.Event
			if msg.CloudEventsVersion() != "" {
				if ok := t.loadCodec(); !ok {
					err := fmt.Errorf("unknown encoding set on transport: %d", t.Encoding)
					log.Printf("failed to load codec: %s", err)
				}
				if respEvent, err = t.codec.Decode(msg); err != nil {
					log.Printf("failed to decode message: %s %v", err, resp)
				}
			}

			if accepted(resp) {
				return respEvent, nil
			}
			return respEvent, fmt.Errorf("error sending cloudevent: %s", status(resp))
		})
	}
	return nil, fmt.Errorf("failed to encode Event into a Message")
}

// SetReceiver implements Transport.SetReceiver
func (t *Transport) SetReceiver(r transport.Receiver) {
	t.Receiver = r
}

// StartReceiver implements Transport.StartReceiver
// NOTE: This is a blocking call.
func (t *Transport) StartReceiver(ctx context.Context) error {
	t.reMu.Lock()
	defer t.reMu.Unlock()

	if t.Handler == nil {
		t.Handler = http.NewServeMux()
	}
	if !t.handlerRegistered {
		// handler.Handle might panic if the user tries to use the same path as the sdk.
		t.Handler.Handle(t.GetPath(), t)
		t.handlerRegistered = true
	}

	addr := fmt.Sprintf(":%d", t.GetPort())
	t.server = &http.Server{
		Addr:    addr,
		Handler: t.Handler,
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	t.realPort = listener.Addr().(*net.TCPAddr).Port

	// Shutdown
	defer func() {
		t.realPort = 0
		t.server.Close()
		t.server = nil
	}()

	errChan := make(chan error, 1)
	go func() {
		errChan <- t.server.Serve(listener)
	}()

	// wait for the server to return or ctx.Done().
	select {
	case <-ctx.Done():
		// Try a gracefully shutdown.
		timeout := DefaultShutdownTimeout
		if t.ShutdownTimeout != nil {
			timeout = *t.ShutdownTimeout
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return t.server.Shutdown(ctx)
	case err := <-errChan:
		return err
	}
}

type eventError struct {
	event *cloudevents.Event
	err   error
}

func httpDo(ctx context.Context, client *http.Client, req *http.Request, fn func(*http.Response, error) (*cloudevents.Event, error)) (*cloudevents.Event, error) {
	// Run the HTTP request in a goroutine and pass the response to fn.
	c := make(chan eventError, 1)
	req = req.WithContext(ctx)
	go func() {
		event, err := fn(client.Do(req))
		c <- eventError{event: event, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case ee := <-c:
		return ee.event, ee.err
	}
}

// accepted is a helper method to understand if the response from the target
// accepted the CloudEvent.
func accepted(resp *http.Response) bool {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true
	}
	return false
}

// status is a helper method to read the response of the target.
func status(resp *http.Response) string {
	status := resp.Status
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("Status[%s] error reading response body: %v", status, err)
	}
	return fmt.Sprintf("Status[%s] %s", status, body)
}

func (t *Transport) invokeReceiver(ctx context.Context, event cloudevents.Event) (*Response, error) {
	ctx, r := observability.NewReporter(ctx, reportReceive)
	resp, err := t.obsInvokeReceiver(ctx, event)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return resp, err
}

func (t *Transport) obsInvokeReceiver(ctx context.Context, event cloudevents.Event) (*Response, error) {
	if t.Receiver != nil {
		// Note: http does not use eventResp.Reason
		eventResp := cloudevents.EventResponse{}
		resp := Response{}

		err := t.Receiver.Receive(ctx, event, &eventResp)
		if err != nil {
			log.Printf("got an error from receiver fn: %s", err.Error())
			resp.StatusCode = http.StatusInternalServerError
			return &resp, err
		}

		if eventResp.Event != nil {
			if t.loadCodec() {
				if m, err := t.codec.Encode(*eventResp.Event); err != nil {
					log.Printf("failed to encode response from receiver fn: %s", err.Error())
				} else if msg, ok := m.(*Message); ok {
					resp.Header = msg.Header
					resp.Body = msg.Body
				}
			} else {
				log.Printf("failed to load codec")
				resp.StatusCode = http.StatusInternalServerError
				return &resp, err
			}
			// Look for a transport response context
			var trx *TransportResponseContext
			if ptrTrx, ok := eventResp.Context.(*TransportResponseContext); ok {
				// found a *TransportResponseContext, use it.
				trx = ptrTrx
			} else if realTrx, ok := eventResp.Context.(TransportResponseContext); ok {
				// found a TransportResponseContext, make it a pointer.
				trx = &realTrx
			}
			// If we found a TransportResponseContext, use it.
			if trx != nil && trx.Header != nil && len(trx.Header) > 0 {
				copyHeaders(trx.Header, resp.Header)
			}
		}

		if eventResp.Status != 0 {
			resp.StatusCode = eventResp.Status
		} else {
			resp.StatusCode = http.StatusAccepted // default is 202 - Accepted
		}
		return &resp, err
	}
	return nil, nil
}

// ServeHTTP implements http.Handler
func (t *Transport) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx, r := observability.NewReporter(req.Context(), reportServeHTTP)

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Printf("failed to handle request: %s %v", err, req)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"Invalid request"}`))
		r.Error()
		return
	}
	msg := &Message{
		Header: req.Header,
		Body:   body,
	}

	if ok := t.loadCodec(); !ok {
		err := fmt.Errorf("unknown encoding set on transport: %d", t.Encoding)
		log.Printf("failed to load codec: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf(`{"error":%q}`, err.Error())))
		r.Error()
		return
	}
	event, err := t.codec.Decode(msg)
	if err != nil {
		log.Printf("failed to decode message: %s %v", err, req)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf(`{"error":%q}`, err.Error())))
		r.Error()
		return
	}

	if req != nil {
		ctx = WithTransportContext(ctx, NewTransportContext(req))
	}

	resp, err := t.invokeReceiver(ctx, *event)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf(`{"error":%q}`, err.Error())))
		r.Error()
		return
	}
	if resp != nil {
		if t.Req != nil {
			copyHeaders(t.Req.Header, w.Header())
		}
		if len(resp.Header) > 0 {
			copyHeaders(resp.Header, w.Header())
		}
		status := http.StatusAccepted
		if resp.StatusCode >= 200 && resp.StatusCode < 600 {
			status = resp.StatusCode
		}
		w.WriteHeader(status)
		if len(resp.Body) > 0 {
			if _, err := w.Write(resp.Body); err != nil {
				r.Error()
				return
			}
		}

		r.OK()
		return
	}

	w.WriteHeader(http.StatusNoContent)
	r.OK()
}

// GetPort returns the port the transport is active on.
// .Port can be set to 0, which means the transport selects a port, GetPort
// allows the transport to report back the selected port.
func (t *Transport) GetPort() int {
	if t.Port != nil && *t.Port == 0 && t.realPort != 0 {
		return t.realPort
	}

	if t.Port != nil && *t.Port >= 0 { // 0 means next open port
		return *t.Port
	}
	return 8080 // default
}

// GetPath returns the path the transport is hosted on. If the path is '/',
// the transport will handle requests on any URI. To discover the true path
// a request was received on, inspect the context from Receive(cxt, ...) with
// TransportContextFrom(ctx).
func (t *Transport) GetPath() string {
	path := strings.TrimSpace(t.Path)
	if len(path) > 0 {
		return path
	}
	return "/" // default
}
