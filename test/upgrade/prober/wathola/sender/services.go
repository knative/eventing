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
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/wavesoftware/go-ensure"
	"knative.dev/eventing/test/upgrade/prober/wathola/config"
	"knative.dev/eventing/test/upgrade/prober/wathola/event"

	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	// ErrEndpointTypeNotSupported is raised if configured endpoint isn't
	// supported by any of the event senders that are registered.
	ErrEndpointTypeNotSupported = errors.New("given endpoint isn't " +
		"supported by any registered event sender")
	log          = config.Log
	senderConfig = &config.Instance.Sender
	eventSenders = make([]EventSender, 0, 1)
)

type sender struct {
	// eventsSent is the number of events successfully sent
	eventsSent int
	// totalRequests is the number of all the send event requests
	totalRequests int
	// unavailablePeriods is an array for non-zero retries for each event
	unavailablePeriods []time.Duration
}

func (s *sender) SendContinually() {
	var shutdownCh = make(chan struct{})
	defer s.sendFinished()

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		sig := <-c
		// sig is a ^C or term, handle it
		log.Infof("%v signal received, closing", sig.String())
		close(shutdownCh)
	}()

	var start time.Time
	retry := 0
	for {
		select {
		case <-shutdownCh:
			// If there is ongoing event transition, we need to push to retries too.
			if retry != 0 {
				s.unavailablePeriods = append(s.unavailablePeriods, time.Since(start))
			}
			return
		default:
		}
		err := s.sendStep()
		if err != nil {
			if retry == 0 {
				start = time.Now()
			}
			log.Warnf("Could not send step event %v, retrying (%d)",
				s.eventsSent, retry)
			log.Debug("Error: ", err)
			retry++
		} else {
			if retry != 0 {
				s.unavailablePeriods = append(s.unavailablePeriods, time.Since(start))
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
	eventSenders = make([]EventSender, 0, 1)
}

// RegisterEventSender will register a EventSender to be used.
func RegisterEventSender(es EventSender) {
	eventSenders = append(eventSenders, es)
}

// SendEvent will send cloud event to given url
func SendEvent(ce cloudevents.Event, endpoint interface{}) error {
	senders := make([]EventSender, 0, len(eventSenders)+1)
	senders = append(senders, eventSenders...)
	if len(senders) == 0 {
		senders = append(senders, httpSender{})
	}
	for _, eventSender := range senders {
		if eventSender.Supports(endpoint) {
			return eventSender.SendEvent(ce, endpoint)
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
	url := endpoint.(string)
	c, err := cloudevents.NewClientHTTP()
	if err != nil {
		return err
	}
	ctx := cloudevents.ContextWithTarget(context.Background(), url)

	result := c.Send(ctx, ce)
	if cloudevents.IsACK(result) {
		return nil
	}
	return result
}

func (s *sender) sendStep() error {
	step := event.Step{Number: s.eventsSent + 1}
	ce := NewCloudEvent(step, event.StepType)
	endpoint := senderConfig.Address
	log.Infof("Sending step event #%v to %#v", step.Number, endpoint)
	err := SendEvent(ce, endpoint)
	// Record every request regardless of the result
	s.totalRequests++
	if err != nil {
		return err
	}
	s.eventsSent++
	return nil
}

func (s *sender) sendFinished() {
	if s.eventsSent == 0 {
		return
	}
	finished := event.Finished{EventsSent: s.eventsSent, TotalRequests: s.totalRequests, UnavailablePeriods: s.unavailablePeriods}
	endpoint := senderConfig.Address
	ce := NewCloudEvent(finished, event.FinishedType)
	log.Infof("Sending finished event (count: %v) to %#v", finished.EventsSent, endpoint)
	ensure.NoError(SendEvent(ce, endpoint))
}
