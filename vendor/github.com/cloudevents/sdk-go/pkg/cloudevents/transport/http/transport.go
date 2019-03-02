package http

import (
	"bytes"
	"context"
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport"
	"io/ioutil"
	"log"
	"net/http"
)

type EncodingSelector func(e cloudevents.Event) Encoding

// type check that this transport message impl matches the contract
var _ transport.Sender = (*Transport)(nil)

// Transport acts as both a http client and a http handler.
type Transport struct {
	Encoding                   Encoding
	DefaultEncodingSelectionFn EncodingSelector

	// Sending
	Client *http.Client
	Req    *http.Request

	// Receiving
	Port     int
	Receiver transport.Receiver

	codec transport.Codec
}

func (t *Transport) loadCodec() bool {
	if t.codec == nil {
		if t.DefaultEncodingSelectionFn != nil && t.Encoding != Default {
			log.Printf("[warn] Transport has a DefaultEncodingSelectionFn set but Encoding is not Default. DefaultEncodingSelectionFn will be ignored.")
		}
		t.codec = &Codec{
			Encoding:                   t.Encoding,
			DefaultEncodingSelectionFn: t.DefaultEncodingSelectionFn,
		}
	}
	return true
}

func (t *Transport) Send(ctx context.Context, event cloudevents.Event) error {
	if t.Client == nil {
		t.Client = &http.Client{}
	}

	var req http.Request
	if t.Req != nil {
		req.Method = t.Req.Method
		req.URL = t.Req.URL
	}

	if ok := t.loadCodec(); !ok {
		return fmt.Errorf("unknown encoding set on transport: %d", t.Encoding)
	}

	msg, err := t.codec.Encode(event)
	if err != nil {
		return err
	}

	// TODO: merge the incoming request with msg, for now just replace.
	if m, ok := msg.(*Message); ok {
		req.Header = m.Header
		req.Body = ioutil.NopCloser(bytes.NewBuffer(m.Body))
		req.ContentLength = int64(len(m.Body))
		return httpDo(ctx, &req, func(resp *http.Response, err error) error {
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if accepted(resp) {
				return nil
			}
			return fmt.Errorf("error sending cloudevent: %s", status(resp))
		})
	}

	return fmt.Errorf("failed to encode Event into a Message")
}

func httpDo(ctx context.Context, req *http.Request, f func(*http.Response, error) error) error {
	// Run the HTTP request in a goroutine and pass the response to f.
	c := make(chan error, 1)
	req = req.WithContext(ctx)
	go func() { c <- f(http.DefaultClient.Do(req)) }()
	select {
	case <-ctx.Done():
		<-c // Wait for f to return.
		return ctx.Err()
	case err := <-c:
		return err
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

// ServeHTTP implements http.Handler
func (t *Transport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("failed to handle request: %s %v", err, r)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"Invalid request"}`))
		return
	}
	msg := &Message{
		Header: r.Header,
		Body:   body,
	}

	if ok := t.loadCodec(); !ok {
		err := fmt.Errorf("unknown encoding set on transport: %d", t.Encoding)
		log.Printf("failed to load codec: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf(`{"error":%q}`, err.Error())))
		return
	}
	event, err := t.codec.Decode(msg)
	if err != nil {
		log.Printf("failed to decode message: %s %v", err, r)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"Decoding Error"}`))
		return
	}

	if t.Receiver != nil {
		go t.Receiver.Receive(*event)
	}

	// TODO: respond correctly based on decode.
	w.WriteHeader(http.StatusNoContent)
}

func (t *Transport) GetPort() int {
	if t.Port > 0 {
		return t.Port
	}
	return 8080 // default
}
