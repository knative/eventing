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
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	ceclient "github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultPort = 8080

	writeTimeout = 1 * time.Minute
)

// Receiver parses Cloud Events, determines if they pass a filter, and sends them to a subscriber.
type Receiver struct {
	logger   *zap.Logger
	client   client.Client
	ceClient ceclient.Client
	ceHTTP   *cehttp.Transport
}

// New creates a new Receiver and its associated MessageReceiver. The caller is responsible for
// Start()ing the returned MessageReceiver.
func New(logger *zap.Logger, client client.Client) (*Receiver, error) {
	ceHTTP, err := cehttp.New(cehttp.WithBinaryEncoding(), cehttp.WithPort(defaultPort))
	if err != nil {
		return nil, err
	}
	ceClient, err := ceclient.New(ceHTTP)
	if err != nil {
		return nil, err
	}

	r := &Receiver{
		logger:   logger,
		client:   client,
		ceClient: ceClient,
		ceHTTP:   ceHTTP,
	}
	err = r.initClient()
	if err != nil {
		return nil, err
	}

	return r, nil
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

// Start begins to receive messages for the receiver.
//
// Only HTTP POST requests to the root path (/) are accepted. If other paths or
// methods are needed, use the HandleRequest method directly with another HTTP
// server.
//
// This method will block until a message is received on the stop channel.
func (r *Receiver) Start(stopCh <-chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- r.ceClient.StartReceiver(ctx, r.serveHTTP)
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
	case <-time.After(writeTimeout):
		return errors.New("timeout shutting down ceClient")
	}
}

func (r *Receiver) serveHTTP(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
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

	triggerRef, err := provisioners.ParseChannel(tctx.Host)
	if err != nil {
		r.logger.Error("Unable to parse host as a trigger", zap.Error(err), zap.String("host", tctx.Host))
		return errors.New("unable to parse host as a Trigger")
	}

	r.logger.Debug("Received message", zap.Any("triggerRef", triggerRef))

	responseEvent, err := r.sendEvent(ctx, tctx, triggerRef, &event)
	if err != nil {
		r.logger.Error("Error sending the event", zap.Error(err))
		return err
	}
	resp.Status = http.StatusAccepted
	resp.Event = responseEvent

	// TODO Add filtered headers (mostly tracing) to the response. We are waiting for CloudEvents
	// SDK to allow this.

	return nil
}

// sendEvent sends an event to a subscriber if the trigger filter passes.
func (r *Receiver) sendEvent(ctx context.Context, tctx cehttp.TransportContext, trigger provisioners.ChannelReference, event *cloudevents.Event) (*cloudevents.Event, error) {
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
	// We could just send the request to this URI regardless, but let's just check to see if it well
	// formed first, that way we can generate better error message if it isn't.
	subscriberURI, err := url.Parse(subscriberURIString)
	if err != nil {
		r.logger.Error("Unable to parse subscriberURI", zap.Error(err), zap.String("subscriberURIString", subscriberURIString))
		return nil, err
	}

	if !r.shouldSendMessage(&t.Spec, event) {
		r.logger.Debug("Message did not pass filter", zap.Any("triggerRef", trigger))
		return nil, nil
	}

	sendingCTX := SendingContext(ctx, tctx, subscriberURI)
	return r.ceHTTP.Send(sendingCTX, *event)
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
	if filterSource != eventingv1alpha1.TriggerAnyFilter && filterSource != actualSource {
		r.logger.Debug("Wrong source", zap.String("trigger.spec.filter.sourceAndType.source", filterSource), zap.String("message.source", actualSource))
		return false
	}
	return true
}
