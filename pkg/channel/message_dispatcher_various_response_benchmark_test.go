/*
Copyright 2022 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package channel_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/transformer"
	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/utils"
)

type fakeMessageHandler struct {
	sendToDestination      bool
	hasDeadLetterSink      bool
	eventExtensions        map[string]string
	header                 http.Header
	body                   string
	fakeResponse           *http.Response
	fakeDeadLetterResponse *http.Response
}

func NewFakeMessageHandler(sendToDestination bool, hasDeadLetterSink bool, eventExtensionsmap map[string]string, header http.Header, body string,
	fakeResponse *http.Response, fakeDeadLetterResponse *http.Response) *fakeMessageHandler {

	fakeMessageHandler := &fakeMessageHandler{
		sendToDestination:      sendToDestination,
		hasDeadLetterSink:      hasDeadLetterSink,
		eventExtensions:        eventExtensionsmap,
		header:                 header,
		body:                   body,
		fakeResponse:           fakeResponse,
		fakeDeadLetterResponse: fakeDeadLetterResponse,
	}
	return fakeMessageHandler
}

func BenchmarkDispatcher_dispatch_ok_response_not_null(b *testing.B) {
	fmh := newSampleResponseAcceptedAndNotNull()
	benchmarkMessageDispatcher(*fmh, b)
}

func BenchmarkDispatcher_dispatch_ok_response_null(b *testing.B) {
	fmh := newSampleResponseAcceptedAndNull()
	benchmarkMessageDispatcher(*fmh, b)
}

func BenchmarkDispatcher_dispatch_fail_response_not_null(b *testing.B) {
	fmh := newSampleResponseStatusInternalServerErrorAndNotNull()
	benchmarkMessageDispatcher(*fmh, b)
}

func newSampleResponseAcceptedAndNotNull() *fakeMessageHandler {
	header := map[string][]string{
		// do-not-forward should not get forwarded.
		"do-not-forward": {"header"},
		"x-request-id":   {"id123"},
		"knative-1":      {"knative-1-value"},
		"knative-2":      {"knative-2-value"},
	}
	body := "destintation"
	eventExtensions := map[string]string{
		"abc": `"ce-abc-value"`,
	}
	fakeResponse := &http.Response{
		StatusCode: http.StatusAccepted,
		Body:       io.NopCloser(bytes.NewBufferString(uuid.NewString())),
	}
	fmh := NewFakeMessageHandler(true, false, eventExtensions, header, body, fakeResponse, nil)
	return fmh
}

func newSampleResponseStatusInternalServerErrorAndNotNull() *fakeMessageHandler {
	header := map[string][]string{
		// do-not-forward should not get forwarded.
		"do-not-forward": {"header"},
		"x-request-id":   {"id123"},
		"knative-1":      {"knative-1-value"},
		"knative-2":      {"knative-2-value"},
	}
	body := "destintation"
	eventExtensions := map[string]string{
		"abc": `"ce-abc-value"`,
	}
	fakeResponse := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(bytes.NewBufferString(uuid.NewString())),
	}
	fmh := NewFakeMessageHandler(true, false, eventExtensions, header, body, fakeResponse, nil)
	return fmh
}

func newSampleResponseAcceptedAndNull() *fakeMessageHandler {
	header := map[string][]string{
		// do-not-forward should not get forwarded.
		"do-not-forward": {"header"},
		"x-request-id":   {"id123"},
		"knative-1":      {"knative-1-value"},
		"knative-2":      {"knative-2-value"},
	}
	body := "destintation"
	eventExtensions := map[string]string{
		"abc": `"ce-abc-value"`,
	}
	fakeResponse := &http.Response{
		StatusCode: http.StatusAccepted,
		Body:       io.NopCloser(bytes.NewBufferString("")),
	}
	fmh := NewFakeMessageHandler(true, false, eventExtensions, header, body, fakeResponse, nil)
	return fmh
}

func benchmarkMessageDispatcher(fmh fakeMessageHandler, b *testing.B) {
	logger := zap.NewNop()
	md := channel.NewMessageDispatcher(logger)
	buf := new(bytes.Buffer)
	buf.ReadFrom(fmh.fakeResponse.Body)
	s := buf.String()

	destHandler := &fakeHttpHandler{
		b:            b,
		response:     fmh.fakeResponse,
		requests:     make([]requestValidation, 0),
		responseBody: s,
	}
	destServer := httptest.NewServer(destHandler)
	destination := getOnlyDomainURL(b, fmh.sendToDestination, destServer.URL)

	var deadLetterSinkHandler *fakeHttpHandler
	var deadLetterSinkServer *httptest.Server
	var deadLetterSink *url.URL
	if fmh.hasDeadLetterSink {
		deadLetterSinkHandler = &fakeHttpHandler{
			b:        b,
			response: fmh.fakeDeadLetterResponse,
			requests: make([]requestValidation, 0),
		}
		deadLetterSinkServer = httptest.NewServer(deadLetterSinkHandler)
		defer deadLetterSinkServer.Close()

		deadLetterSink = getOnlyDomainURL(b, true, deadLetterSinkServer.URL)
	}

	event := test.FullEvent()
	event.SetID(uuid.New().String())
	event.SetType("testtype")
	event.SetSource("testsource")
	for n, v := range fmh.eventExtensions {
		event.SetExtension(n, v)
	}
	event.SetData(cloudevents.ApplicationJSON, fmh.body)

	ctx := context.Background()

	message := binding.ToMessage(&event)
	var err error
	ev, err := binding.ToEvent(ctx, message, binding.Transformers{transformer.AddTimeNow})
	if err != nil {
		b.Fatal(err)
	}
	message = binding.ToMessage(ev)
	finishInvoked := 0
	message = binding.WithFinish(message, func(err error) {
		finishInvoked++
	})

	var headers http.Header = nil
	if fmh.header != nil {
		headers = utils.PassThroughHeaders(fmh.header)
	}
	// Start the bench
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = md.DispatchMessage(ctx, message, headers, destination, nil, deadLetterSink)
	}

}

type requestValidation struct {
	Host    string
	Headers http.Header
	Body    string
}

type fakeHttpHandler struct {
	b            *testing.B
	response     *http.Response
	requests     []requestValidation
	responseBody string
}

func (f *fakeHttpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	// Make a copy of the request.
	body, err := io.ReadAll(r.Body)
	if err != nil {

		f.b.Error("Failed to read the request body")
	}
	f.requests = append(f.requests, requestValidation{
		Host:    r.Host,
		Headers: r.Header,
		Body:    string(body),
	})

	if f.response != nil {
		for h, vs := range f.response.Header {
			for _, v := range vs {
				w.Header().Add(h, v)
			}
		}
		w.WriteHeader(f.response.StatusCode)
		w.Write([]byte(f.responseBody))
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(""))
	}
}

func getOnlyDomainURL(b *testing.B, shouldSend bool, serverURL string) *url.URL {
	if shouldSend {
		server, err := url.Parse(serverURL)
		if err != nil {
			b.Errorf("Bad serverURL: %q", serverURL)
		}
		return &url.URL{
			Host: server.Host,
		}
	}
	return nil
}
