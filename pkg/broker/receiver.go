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

package broker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultPort = 8080
)

// Receiver parses Cloud Events, determines if they pass a filter, and sends them to a subscriber.
type Receiver struct {
	logger *zap.Logger
	client client.Client

	port int

	httpClient HTTPDoer
	codec      cehttp.Codec
}

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

var _ HTTPDoer = &http.Client{}

// New creates a new Receiver and its associated MessageReceiver. The caller is responsible for
// Start()ing the returned MessageReceiver.
func New(logger *zap.Logger, client client.Client) *Receiver {
	r := &Receiver{
		logger: logger,
		client: client,

		port: defaultPort,

		httpClient: &http.Client{},
		codec: cehttp.Codec{
			Encoding: cehttp.BinaryV01,
		},
	}
	return r
}

var _ http.Handler = &Receiver{}

// Start begins to receive messages for the receiver.
//
// Only HTTP POST requests to the root path (/) are accepted. If other paths or
// methods are needed, use the HandleRequest method directly with another HTTP
// server.
//
// This method will block until a message is received on the stop channel.
func (r *Receiver) Start(stopCh <-chan struct{}) error {
	svr := r.start()
	defer r.stop(svr)

	<-stopCh
	return nil
}

func (r *Receiver) start() *http.Server {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", r.port),
		Handler: r,
	}
	r.logger.Info("Starting web server", zap.String("addr", srv.Addr))
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			r.logger.Error("HttpServer: ListenAndServe() error", zap.Error(err))
		}
	}()
	return srv
}

func (r *Receiver) stop(srv *http.Server) {
	r.logger.Info("Shutdown web server")
	if err := srv.Shutdown(nil); err != nil {
		r.logger.Error("Error shutting down the HTTP Server", zap.Error(err))
	}
}

func (r *Receiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	triggerRef, err := provisioners.ParseChannel(req.Host)
	if err != nil {
		r.logger.Error("Unable to parse host as a trigger", zap.Error(err), zap.String("host", req.Host))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"Bad Host"}`))
		return
	}

	event, err := r.decodeHTTPRequest(req)
	if err != nil {
		r.logger.Error("Error decoding HTTP Request", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	responseEvent, err := r.sendEvent(triggerRef, event)
	if err != nil {
		r.logger.Error("Error sending the event", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if responseEvent == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	encodedEvent, err := r.codec.Encode(*event)
	if err != nil {
		r.logger.Error("Error encoding the response event", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	msg, ok := encodedEvent.(*cehttp.Message)
	if !ok {
		r.logger.Error("Error casting the encoded response event", zap.Error(err), zap.Any("encodedEvent", reflect.TypeOf(encodedEvent)))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	for n, v := range msg.Header {
		w.Header().Del(n)
		for _, s := range v {
			w.Header().Add(n, s)
		}
	}
	w.WriteHeader(http.StatusAccepted)
	_, err = w.Write(msg.Body)
	if err != nil {
		r.logger.Error("Error writing the response body", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (r *Receiver) decodeHTTPRequest(req *http.Request) (*cloudevents.Event, error) {
	return r.decodeHTTP(req.Header, req.Body)
}

func (r *Receiver) decodeHTTPResponse(resp *http.Response) (*cloudevents.Event, error) {
	// The HTTP Response could be anything, so just assume that if it does not parse as a
	// CloudEvent, then it isn't a CloudEvent.
	e, _ := r.decodeHTTP(resp.Header, resp.Body)
	return e, nil
}

func (r *Receiver) decodeHTTP(headers http.Header, bodyReadCloser io.ReadCloser) (*cloudevents.Event, error) {
	body, err := ioutil.ReadAll(bodyReadCloser)
	if err != nil {
		return nil, err
	}
	msg := &cehttp.Message{
		Header: headers,
		Body:   body,
	}

	return r.codec.Decode(msg)
}

// sendEvent sends an event to a subscriber if the trigger filter passes.
func (r *Receiver) sendEvent(trigger provisioners.ChannelReference, event *cloudevents.Event) (*cloudevents.Event, error) {
	r.logger.Debug("Received message", zap.Any("triggerRef", trigger))
	ctx := context.Background()

	t, err := r.getTrigger(ctx, trigger)
	if err != nil {
		r.logger.Info("Unable to get the Trigger", zap.Error(err), zap.Any("triggerRef", trigger))
		return nil, err
	}

	subscriberURIString := t.Status.SubscriberURI
	if subscriberURIString == "" {
		r.logger.Error("Unable to read subscriberURI")
		return nil, errors.New("unable to read subscriberURI")
	}
	subscriberURI, err := url.Parse(subscriberURIString)
	if err != nil {
		r.logger.Error("Unable to parse subscriberURI", zap.Error(err), zap.String("subscriberURIString", subscriberURIString))
		return nil, err
	}

	if !r.shouldSendMessage(&t.Spec, event) {
		r.logger.Debug("Message did not pass filter", zap.Any("triggerRef", trigger))
		return nil, nil
	}

	return r.dispatch(event, subscriberURI)
}

func (r *Receiver) dispatch(event *cloudevents.Event, uri *url.URL) (*cloudevents.Event, error) {
	encodedEvent, err := r.codec.Encode(*event)
	if err != nil {
		return nil, err
	}
	msg, ok := encodedEvent.(*cehttp.Message)
	if !ok {
		return nil, errors.New("msg was not a cehttp.Message")
	}

	req, err := http.NewRequest(http.MethodPost, uri.String(), bytes.NewReader(msg.Body))
	if err != nil {
		return nil, err
	}

	req.Header = msg.Header
	res, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res == nil {
		// I don't think this is actually reachable with http.Client.Do(), but just to be sure we
		// check anyway.
		return nil, errors.New("non-error nil result from http.Client.Do()")
	}

	defer res.Body.Close()
	if isFailure(res.StatusCode) {
		// reject non-successful responses
		return nil, fmt.Errorf("unexpected HTTP response, expected 2xx, got %d", res.StatusCode)
	}

	return r.decodeHTTPResponse(res)
}

// isFailure returns true if the status code is not a successful HTTP status.
func isFailure(statusCode int) bool {
	return statusCode < http.StatusOK /* 200 */ ||
		statusCode >= http.StatusMultipleChoices /* 300 */
}

// Initialize the client. Mainly intended to load stuff in its cache.
func (r *Receiver) initClient() error {
	// We list triggers so that we do not drop messages. Otherwise, on receiving an event, it
	// may not find the Trigger and would return an error.
	opts := &client.ListOptions{}
	tl := &eventingv1alpha1.TriggerList{}
	if err := r.client.List(context.TODO(), opts, tl); err != nil {
		return err
	}
	return nil
}

func (r *Receiver) getTrigger(ctx context.Context, ref provisioners.ChannelReference) (*eventingv1alpha1.Trigger, error) {
	t := &eventingv1alpha1.Trigger{}
	err := r.client.Get(ctx,
		types.NamespacedName{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		t)
	return t, err
}

// shouldSendMessage determines whether message 'm' should be sent based on the triggerSpec 'ts'.
// Currently it supports exact matching on type and/or source of events.
func (r *Receiver) shouldSendMessage(ts *eventingv1alpha1.TriggerSpec, event *cloudevents.Event) bool {
	if ts.Filter == nil || ts.Filter.SourceAndType == nil {
		r.logger.Error("No filter specified")
		return false
	}
	filterType := ts.Filter.SourceAndType.Type
	if filterType != eventingv1alpha1.TriggerAnyFilter && filterType != event.Type() {
		r.logger.Debug("Wrong type", zap.String("trigger.spec.filter.sourceAndType.type", filterType), zap.String("event.Type()", event.Type()))
		return false
	}
	filterSource := ts.Filter.SourceAndType.Source
	s := event.Context.AsV01().Source
	actualSource := s.String()
	//actualSource := event.Context.AsV01().Source.String()
	if filterSource != eventingv1alpha1.TriggerAnyFilter && filterSource != actualSource {
		r.logger.Debug("Wrong source", zap.String("trigger.spec.filter.sourceAndType.source", filterSource), zap.String("message.source", actualSource))
		return false
	}
	return true
}
