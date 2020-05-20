/*
 * Copyright 2020 The Knative Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package sender

import (
	"context"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/wavesoftware/go-ensure"
	"knative.dev/eventing/test/upgrade/prober/wathola/config"
	"knative.dev/eventing/test/upgrade/prober/wathola/event"

	"os"
	"os/signal"
	"syscall"
	"time"
)

var log = config.Log
var senderConfig = &config.Instance.Sender

type sender struct {
	counter int
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

	for {
		select {
		case <-shutdownCh:
			return
		default:
		}
		err := s.sendStep()
		if err != nil {
			log.Warnf("Could not send step event, retry in %v", senderConfig.Cooldown)
			time.Sleep(senderConfig.Cooldown)
		} else {
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
	e.SetTime(time.Now())
	err = e.SetData(cloudevents.ApplicationJSON, data)
	ensure.NoError(err)
	errs := e.Validate()
	if errs != nil {
		ensure.NoError(errs)
	}
	return e
}

// SendEvent will send cloud event to given url
func SendEvent(e cloudevents.Event, url string) error {
	c, err := cloudevents.NewDefaultClient()
	if err != nil {
		return err
	}
	ctx := cloudevents.ContextWithTarget(context.Background(), url)

	result := c.Send(ctx, e)
	if cloudevents.IsACK(result) {
		return nil
	}
	return result
}

func (s *sender) sendStep() error {
	step := event.Step{Number: s.counter + 1}
	ce := NewCloudEvent(step, event.StepType)
	url := senderConfig.Address
	log.Infof("Sending step event #%v to %s", step.Number, url)
	err := SendEvent(ce, url)
	if err != nil {
		return err
	}
	s.counter++
	return nil
}

func (s *sender) sendFinished() {
	if s.counter == 0 {
		return
	}
	finished := event.Finished{Count: s.counter}
	url := senderConfig.Address
	ce := NewCloudEvent(finished, event.FinishedType)
	log.Infof("Sending finished event (count: %v) to %s", finished.Count, url)
	ensure.NoError(SendEvent(ce, url))
}
