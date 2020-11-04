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

package receiver

import (
	"context"
	"encoding/json"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/wavesoftware/go-ensure"
	"knative.dev/eventing/test/upgrade/prober/wathola/client"
	"knative.dev/eventing/test/upgrade/prober/wathola/config"
	"knative.dev/eventing/test/upgrade/prober/wathola/event"

	"net/http"
)

var (
	log       = config.Log
	Canceling = make(chan context.CancelFunc, 1)
)

// New creates new Receiver
func New() Receiver {
	config.ReadIfPresent()
	errors := event.NewErrorStore()
	stepsStore := event.NewStepsStore(errors)
	finishedStore := event.NewFinishedStore(stepsStore, errors)
	r := newReceiver(stepsStore, finishedStore)
	return r
}

func (r receiver) Receive() {
	port := config.Instance.Receiver.Port
	client.Receive(port, Canceling, r.receiveEvent, r.reportMiddleware)
}

func (r receiver) receiveEvent(e cloudevents.Event) {
	// do something with event.Context and event.Data (via event.DataAs(foo)
	t := e.Context.GetType()
	if t == event.StepType {
		step := &event.Step{}
		err := e.DataAs(step)
		if err != nil {
			log.Fatal(err)
		}
		r.step.RegisterStep(step)
	}
	if t == event.FinishedType {
		finished := &event.Finished{}
		err := e.DataAs(finished)
		if err != nil {
			log.Fatal(err)
		}
		r.finished.RegisterFinished(finished)
	}
}

func (r *receiver) reportMiddleware(next http.Handler) http.Handler {
	return &reportHandler{
		next:     next,
		receiver: r,
	}
}

type receiver struct {
	step     event.StepsStore
	finished event.FinishedStore
}

func newReceiver(step event.StepsStore, finished event.FinishedStore) *receiver {
	r := &receiver{
		step:     step,
		finished: finished,
	}
	return r
}

type reportHandler struct {
	next     http.Handler
	receiver *receiver
}

func (r reportHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.RequestURI == "/report" {
		s := r.receiver.finished.State()
		errs := r.receiver.finished.Thrown()
		events := r.receiver.step.Count()
		sj := &Report{
			State:  stateToString(s),
			Events: events,
			Thrown: errs,
		}
		b, err := json.Marshal(sj)
		ensure.NoError(err)
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
		_, err = rw.Write(b)
		ensure.NoError(err)
	} else {
		r.next.ServeHTTP(rw, req)
	}
}

func stateToString(state event.State) string {
	switch state {
	case event.Active:
		return "active"
	case event.Success:
		return "success"
	case event.Failed:
		return "failed"
	default:
		panic(fmt.Sprintf("unknown state: %v", state))
	}
}
