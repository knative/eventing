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

package event

import (
	"fmt"
	"sync"
	"time"

	"knative.dev/eventing/test/upgrade/prober/wathola/config"
)

const FinishedEventPrefix = "finish event"

var mutex = sync.RWMutex{}
var lastProgressReport = time.Now()

type thrownTypes struct {
	missing     []thrown
	duplicated  []thrown
	unexpected  []thrown
	unavailable []thrown
}

// ErrorStore contains errors that was thrown
type ErrorStore struct {
	state  State
	thrown thrownTypes
}

// NewErrorStore creates a new error store
func NewErrorStore() *ErrorStore {
	return &ErrorStore{
		state: Active,
		thrown: thrownTypes{
			missing:     make([]thrown, 0),
			duplicated:  make([]thrown, 0),
			unexpected:  make([]thrown, 0),
			unavailable: make([]thrown, 0),
		},
	}
}

// NewStepsStore creates StepsStore
func NewStepsStore(errors *ErrorStore) StepsStore {
	return &stepStore{
		store:  make(map[int]int),
		errors: errors,
	}
}

// NewFinishedStore creates FinishedStore
func NewFinishedStore(steps StepsStore, errors *ErrorStore) FinishedStore {
	return &finishedStore{
		received:      0,
		eventsSent:    -1,
		totalRequests: 0,
		steps:         steps,
		errors:        errors,
	}
}

func (s *stepStore) RegisterStep(step *Step) {
	mutex.Lock()
	if times, found := s.store[step.Number]; found {
		s.errors.throwDuplicated(
			"event #%d received %d times, but should be received only once",
			step.Number, times+1)
	} else {
		s.store[step.Number] = 0
	}
	s.store[step.Number]++
	mutex.Unlock()
	log.Debugf("event #%d received", step.Number)
	s.reportProgress()
}

func (s *stepStore) Count() int {
	return len(s.store)
}

func (f *finishedStore) RegisterFinished(finished *Finished) {
	if f.received > 0 {
		f.errors.throwDuplicated(
			"%s should be received only once, received %d",
			FinishedEventPrefix, f.received+1)
		// We don't want to record all failures again.
		return
	}
	f.received++
	f.eventsSent = finished.EventsSent
	f.totalRequests = finished.TotalRequests
	log.Infof("finish event received, expecting %d event ware propagated", finished.EventsSent)
	d := config.Instance.Receiver.Teardown.Duration
	log.Infof("waiting additional %v to be sure all events came", d)
	time.Sleep(d)
	receivedEvents := f.steps.Count()

	if receivedEvents != finished.EventsSent &&
		// If sending was interrupted, tolerate one more received
		// event as there's no way to check if the last event is delivered or not.
		!(finished.SendingInterrupted && receivedEvents == finished.EventsSent+1) {
		f.errors.throwUnexpected("expecting to have %v unique events received, "+
			"but received %v unique events", finished.EventsSent, receivedEvents)
		f.reportViolations(finished)
		f.errors.state = Failed
	} else {
		log.Infof("properly received %d unique events", receivedEvents)
		f.errors.state = Success
	}
	// check down time
	for _, unavailablePeriod := range finished.UnavailablePeriods {
		if unavailablePeriod.Period > config.Instance.Receiver.Errors.UnavailablePeriodToReport {
			f.errors.throwUnavail("event #%d had unavailable period %v which is over down time limit of %v, last error %v",
				unavailablePeriod.Step.Number,
				unavailablePeriod.Period,
				config.Instance.Receiver.Errors.UnavailablePeriodToReport,
				unavailablePeriod.LastErr)
			f.errors.state = Failed
		}
		log.Infof("detecting unavailable time %v", unavailablePeriod)
	}
}

func (f *finishedStore) TotalRequests() int {
	return f.totalRequests
}

func (f *finishedStore) State() State {
	return f.errors.state
}

func (f *finishedStore) DuplicatedThrown() []string {
	return asStrings(f.errors.thrown.duplicated)
}

func (f *finishedStore) MissingThrown() []string {
	return asStrings(f.errors.thrown.missing)
}

func (f *finishedStore) UnexpectedThrown() []string {
	return asStrings(f.errors.thrown.unexpected)
}

func (f *finishedStore) UnavailableThrown() []string {
	return asStrings(f.errors.thrown.unavailable)
}

func asStrings(errThrown []thrown) []string {
	msgs := make([]string, 0)
	for _, t := range errThrown {
		errMsg := fmt.Sprintf(t.format, t.args...)
		msgs = append(msgs, errMsg)
	}
	return msgs
}

func (f *finishedStore) reportViolations(finished *Finished) {
	steps := f.steps.(*stepStore)
	for eventNo := 1; eventNo <= finished.EventsSent; eventNo++ {
		times := steps.store[eventNo]
		if times != 1 {
			throwMethod := f.errors.throwMissing
			if times > 1 {
				throwMethod = f.errors.throwDuplicated
			}
			throwMethod("event #%v should be received once, but was received %v times",
				eventNo, times)
		}
	}
}

func (s *stepStore) reportProgress() {
	if lastProgressReport.Add(config.Instance.Receiver.Progress.Duration).Before(time.Now()) {
		lastProgressReport = time.Now()
		log.Infof("collected %v unique events", s.Count())
	}
}

func (e *ErrorStore) throwDuplicated(format string, args ...interface{}) {
	e.thrown.duplicated = e.appendThrown(e.thrown.duplicated, format, args...)
}

func (e *ErrorStore) throwMissing(format string, args ...interface{}) {
	e.thrown.missing = e.appendThrown(e.thrown.missing, format, args...)
}

func (e *ErrorStore) throwUnexpected(format string, args ...interface{}) {
	e.thrown.unexpected = e.appendThrown(e.thrown.unexpected, format, args...)
}

func (e *ErrorStore) throwUnavail(format string, args ...interface{}) {
	e.thrown.unavailable = e.appendThrown(e.thrown.unavailable, format, args...)
}

func (e *ErrorStore) appendThrown(errThrown []thrown, format string, args ...interface{}) []thrown {
	t := thrown{
		format: format,
		args:   args,
	}
	log.Errorf(t.format, t.args...)
	return append(errThrown, t)
}

type stepStore struct {
	store  map[int]int
	errors *ErrorStore
}

type finishedStore struct {
	received      int
	eventsSent    int
	totalRequests int
	errors        *ErrorStore
	steps         StepsStore
}

type thrown struct {
	format string
	args   []interface{}
}
