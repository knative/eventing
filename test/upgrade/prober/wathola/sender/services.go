/*
Copyright 2021 The Knative Authors

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

package sender

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cloudeventshttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/wavesoftware/go-ensure"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"

	"knative.dev/eventing/test/upgrade/prober/wathola/config"
	"knative.dev/eventing/test/upgrade/prober/wathola/event"
)

const (
	Name = "wathola-sender"
)

var (
	// ErrEndpointTypeNotSupported is raised if configured endpoint isn't
	// supported by any of the event senders that are registered.
	ErrEndpointTypeNotSupported = errors.New("given endpoint isn't " +
		"supported by any registered event sender")
	log                     = config.Log
	senderConfig            = &config.Instance.Sender
	eventSendersWithContext = make([]EventSenderWithContext, 0, 1)
)

type sender struct {
	// eventsSent is the number of events successfully sent
	eventsSent int
	// totalRequests is the number of all the send event requests
	totalRequests int
	// unavailablePeriods is an array for non-zero retries for each event
	unavailablePeriods []event.UnavailablePeriod
	// sendingInterrupted indicates whether sending last event was interrupted by shutdown
	sendingInterrupted bool
}

func (s *sender) SendContinually() {
	var shutdownCh = make(chan struct{})
	defer func() {
		s.sendFinished()
	}()

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		sig := <-c
		// sig is a ^C or term, handle it
		log.Infof("%v signal received, closing", sig.String())
		close(shutdownCh)
	}()

	var start time.Time
	var currentStep *event.Step
	var err error
	retry := 0
	var lastErr error
	for {
		select {
		case <-shutdownCh:
			// If there is ongoing event transition, we need to push to retries too.
			if retry != 0 {
				s.unavailablePeriods = append(s.unavailablePeriods, event.UnavailablePeriod{
					Step:    currentStep,
					Period:  time.Since(start),
					LastErr: err.Error(),
				})
				s.sendingInterrupted = true
			}
			return
		default:
		}
		currentStep, err = s.sendStep()
		if err != nil {
			if retry == 0 {
				start = time.Now()
			}
			log.Warnf("Could not send step event %v, retrying (%d): %v",
				currentStep.Number, retry, err)
			retry++
			lastErr = err
		} else {
			if retry != 0 {
				s.unavailablePeriods = append(s.unavailablePeriods, event.UnavailablePeriod{
					Step:    currentStep,
					Period:  time.Since(start),
					LastErr: lastErr.Error(),
				})
				log.Warnf("Event sent after %v retries", retry)
				retry = 0
			}
			time.Sleep(senderConfig.Interval)
		}
	}
}

// NewCloudEvent creates a new cloud event
func NewCloudEvent(data interface{}, typ string) cloudevents.Event {
	e := cloudevents.NewEvent()
	e.SetDataContentType("application/json")
	e.SetType(typ)
	host, err := os.Hostname()
	ensure.NoError(err)
	e.SetSource(fmt.Sprintf("knative://%s/wathola/sender", host))
	e.SetID(NewEventID())
	e.SetTime(time.Now().UTC())
	err = e.SetData(cloudevents.ApplicationJSON, data)
	ensure.NoError(err)
	errs := e.Validate()
	if errs != nil {
		ensure.NoError(errs)
	}
	return e
}

// ResetEventSenders will reset configured event senders to defaults.
func ResetEventSenders() {
	eventSendersWithContext = make([]EventSenderWithContext, 0, 1)
}

// RegisterEventSenderWithContext will register EventSenderWithContext to be used.
func RegisterEventSenderWithContext(es EventSenderWithContext) {
	eventSendersWithContext = append(eventSendersWithContext, es)
}

// SendEvent will send cloud event to given url
func SendEvent(ctx context.Context, ce cloudevents.Event, endpoint interface{}) error {
	sendersWithCtx := make([]EventSenderWithContext, 0, len(eventSendersWithContext)+1)
	sendersWithCtx = append(sendersWithCtx, eventSendersWithContext...)
	if len(eventSendersWithContext) == 0 {
		sendersWithCtx = append(sendersWithCtx, httpSender{})
	}
	for _, eventSender := range sendersWithCtx {
		if eventSender.Supports(endpoint) {
			return eventSender.SendEventWithContext(ctx, ce, endpoint)
		}
	}
	return fmt.Errorf("%w: endpoint is %#v", ErrEndpointTypeNotSupported, endpoint)
}

type httpSender struct{}

func (h httpSender) Supports(endpoint interface{}) bool {
	switch url := endpoint.(type) {
	default:
		return false
	case string:
		return strings.HasPrefix(url, "http://") ||
			strings.HasPrefix(url, "https://")
	}
}

func (h httpSender) SendEvent(ce cloudevents.Event, endpoint interface{}) error {
	return h.SendEventWithContext(context.Background(), ce, endpoint)
}

func (h httpSender) SendEventWithContext(ctx context.Context, ce cloudevents.Event, endpoint interface{}) error {
	url := endpoint.(string)
	opts := []cloudeventshttp.Option{
		cloudevents.WithRoundTripper(&ochttp.Transport{
			Propagation: tracecontextb3.TraceContextEgress,
		}),
	}
	c, err := cloudevents.NewClientHTTP(opts...)
	if err != nil {
		return err
	}
	ctxWithTarget := cloudevents.ContextWithTarget(ctx, url)
	result := c.Send(ctxWithTarget, ce)
	if cloudevents.IsACK(result) {
		return nil
	}
	return result
}

func (s *sender) sendStep() (*event.Step, error) {
	step := event.Step{Number: s.eventsSent + 1}
	ce := NewCloudEvent(step, event.StepType)
	ctx, span := PopulateSpanWithEvent(context.Background(), ce, Name)
	defer span.End()
	span.AddAttributes(trace.Int64Attribute("step", int64(step.Number)))
	endpoint := senderConfig.Address
	log.Infof("Sending step event #%v to %#v", step.Number, endpoint)
	err := SendEvent(ctx, ce, endpoint)
	// Record every request regardless of the result
	s.totalRequests++
	if err != nil {
		return &step, err
	}
	s.eventsSent++
	return &step, nil
}

func (s *sender) sendFinished() {
	if s.eventsSent == 0 {
		return
	}
	finished := event.Finished{
		EventsSent:         s.eventsSent,
		TotalRequests:      s.totalRequests,
		UnavailablePeriods: s.unavailablePeriods,
		SendingInterrupted: s.sendingInterrupted,
	}
	endpoint := senderConfig.Address
	ce := NewCloudEvent(finished, event.FinishedType)
	ctx, span := PopulateSpanWithEvent(context.Background(), ce, Name)
	defer span.End()
	log.Infof("Sending finished event (count: %v) to %#v", finished.EventsSent, endpoint)
	ensure.NoError(SendEvent(ctx, ce, endpoint))
}

func PopulateSpanWithEvent(ctx context.Context, ce cloudevents.Event, spanName string) (context.Context, *trace.Span) {
	ctxWithSpan, span := trace.StartSpan(ctx, spanName)
	span.AddAttributes(
		trace.StringAttribute("cloudevents.type", ce.Type()),
		trace.StringAttribute("cloudevents.id", ce.ID()),
		trace.StringAttribute("target", config.Instance.Forwarder.Target),
	)
	return ctxWithSpan, span
}
