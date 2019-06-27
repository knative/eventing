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
	"fmt"
	"net/http"
	"net/url"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/reconciler/trigger/path"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.uber.org/zap"
	"knative.dev/pkg/tracing"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	writeTimeout = 1 * time.Minute
)

// Receiver parses Cloud Events, determines if they pass a filter, and sends them to a subscriber.
type Receiver struct {
	logger   *zap.Logger
	client   client.Client
	ceClient cloudevents.Client
}

// New creates a new Receiver and its associated MessageReceiver. The caller is responsible for
// Start()ing the returned MessageReceiver.
func New(logger *zap.Logger, client client.Client) (*Receiver, error) {
	httpTransport, err := cloudevents.NewHTTPTransport(cloudevents.WithBinaryEncoding(), cehttp.WithMiddleware(tracing.HTTPSpanMiddleware))
	if err != nil {
		return nil, err
	}
	ceClient, err := cloudevents.NewClient(httpTransport, cloudevents.WithTimeNow(), cloudevents.WithUUIDs())
	if err != nil {
		return nil, err
	}

	r := &Receiver{
		logger:   logger,
		client:   client,
		ceClient: ceClient,
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
	tctx := cloudevents.HTTPTransportContextFrom(ctx)
	if tctx.Method != http.MethodPost {
		resp.Status = http.StatusMethodNotAllowed
		return nil
	}

	// tctx.URI is actually the path...
	triggerRef, err := path.Parse(tctx.URI)
	if err != nil {
		r.logger.Info("Unable to parse path as a trigger", zap.Error(err), zap.String("path", tctx.URI))
		return errors.New("unable to parse path as a Trigger")
	}

	// Remove the TTL attribute that is used by the Broker.
	originalV2 := event.Context.AsV02()
	ttl, ttlKey := GetTTL(event.Context)
	if ttl == nil {
		// Only messages sent by the Broker should be here. If the attribute isn't here, then the
		// event wasn't sent by the Broker, so we can drop it.
		r.logger.Warn("No TTL seen, dropping", zap.Any("triggerRef", triggerRef), zap.Any("event", event))
		// This doesn't return an error because normally this function is called by a Channel, which
		// will retry all non-2XX responses. If we return an error from this function, then the
		// framework returns a 500 to the caller, so the Channel would send this repeatedly.
		return nil
	}
	delete(originalV2.Extensions, ttlKey)
	event.Context = originalV2

	r.logger.Debug("Received message", zap.Any("triggerRef", triggerRef))

	responseEvent, err := r.sendEvent(ctx, tctx, triggerRef, &event)
	if err != nil {
		r.logger.Error("Error sending the event", zap.Error(err))
		return err
	}

	resp.Status = http.StatusAccepted
	if responseEvent == nil {
		return nil
	}

	// Reattach the TTL (with the same value) to the response event before sending it to the Broker.
	responseEvent.Context, err = SetTTL(responseEvent.Context, ttl)
	if err != nil {
		return err
	}
	resp.Event = responseEvent
	resp.Context = &cloudevents.HTTPTransportResponseContext{
		Header: extractPassThroughHeaders(tctx),
	}

	return nil
}

// sendEvent sends an event to a subscriber if the trigger filter passes.
func (r *Receiver) sendEvent(ctx context.Context, tctx cloudevents.HTTPTransportContext, trigger path.NamespacedNameUID, event *cloudevents.Event) (*cloudevents.Event, error) {
	t, err := r.getTrigger(ctx, trigger)
	if err != nil {
		r.logger.Info("Unable to get the Trigger", zap.Error(err), zap.Any("triggerRef", trigger))
		return nil, err
	}

	// Set up the metrics context
	ctx, _ = tag.New(ctx,
		tag.Insert(TagTrigger, trigger.String()),
		tag.Insert(TagBroker, fmt.Sprintf("%s/%s", trigger.Namespace, t.Spec.Broker)),
	)
	// Record event count and filtering time
	startTS := time.Now()
	defer func() {
		dispatchTimeMS := int64(time.Now().Sub(startTS) / time.Millisecond)
		stats.Record(ctx, MeasureTriggerDispatchTime.M(dispatchTimeMS))
		stats.Record(ctx, MeasureTriggerEventsTotal.M(1))
	}()

	subscriberURIString := t.Status.SubscriberURI
	if subscriberURIString == "" {
		ctx, _ = tag.New(ctx, tag.Upsert(TagResult, "error"))
		return nil, errors.New("unable to read subscriberURI")
	}
	// We could just send the request to this URI regardless, but let's just check to see if it well
	// formed first, that way we can generate better error message if it isn't.
	subscriberURI, err := url.Parse(subscriberURIString)
	if err != nil {
		r.logger.Error("Unable to parse subscriberURI", zap.Error(err), zap.String("subscriberURIString", subscriberURIString))
		ctx, _ = tag.New(ctx, tag.Upsert(TagResult, "error"))
		return nil, err
	}

	if !r.shouldSendMessage(ctx, &t.Spec, event) {
		r.logger.Debug("Message did not pass filter", zap.Any("triggerRef", trigger))
		ctx, _ = tag.New(ctx, tag.Upsert(TagResult, "drop"))
		return nil, nil
	}

	sendingCTX := SendingContext(ctx, tctx, subscriberURI)
	replyEvent, err := r.ceClient.Send(sendingCTX, *event)
	if err == nil {
		ctx, _ = tag.New(ctx, tag.Upsert(TagResult, "accept"))
	} else {
		ctx, _ = tag.New(ctx, tag.Upsert(TagResult, "error"))
	}
	return replyEvent, err
}

func (r *Receiver) getTrigger(ctx context.Context, ref path.NamespacedNameUID) (*eventingv1alpha1.Trigger, error) {
	t := &eventingv1alpha1.Trigger{}
	err := r.client.Get(ctx, ref.NamespacedName, t)
	if err != nil {
		return nil, err
	}
	if t.UID != ref.UID {
		return nil, fmt.Errorf("trigger had a different UID. From ref '%s'. From Kubernetes '%s'", ref.UID, t.UID)
	}
	return t, nil
}

// shouldSendMessage determines whether message 'm' should be sent based on the triggerSpec 'ts'.
// Currently it supports exact matching on type and/or source of events.
func (r *Receiver) shouldSendMessage(ctx context.Context, ts *eventingv1alpha1.TriggerSpec, event *cloudevents.Event) bool {
	if ts.Filter == nil || ts.Filter.SourceAndType == nil {
		r.logger.Error("No filter specified")
		ctx, _ = tag.New(ctx, tag.Upsert(TagFilterResult, "empty-fail"))
		return false
	}

	// Record event count and filtering time
	startTS := time.Now()
	defer func() {
		filterTimeMS := int64(time.Now().Sub(startTS) / time.Millisecond)
		stats.Record(ctx, MeasureTriggerFilterTime.M(filterTimeMS))
	}()

	filterType := ts.Filter.SourceAndType.Type
	if filterType != eventingv1alpha1.TriggerAnyFilter && filterType != event.Type() {
		r.logger.Debug("Wrong type", zap.String("trigger.spec.filter.sourceAndType.type", filterType), zap.String("event.Type()", event.Type()))
		ctx, _ = tag.New(ctx, tag.Upsert(TagFilterResult, "fail"))
		return false
	}
	filterSource := ts.Filter.SourceAndType.Source
	s := event.Context.AsV01().Source
	actualSource := s.String()
	if filterSource != eventingv1alpha1.TriggerAnyFilter && filterSource != actualSource {
		r.logger.Debug("Wrong source", zap.String("trigger.spec.filter.sourceAndType.source", filterSource), zap.String("message.source", actualSource))
		ctx, _ = tag.New(ctx, tag.Upsert(TagFilterResult, "fail"))

		return false
	}

	ctx, _ = tag.New(ctx, tag.Upsert(TagFilterResult, "pass"))
	return true
}
