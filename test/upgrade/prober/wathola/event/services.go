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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/openzipkin/zipkin-go/model"
	"knative.dev/eventing/test/upgrade/prober/wathola/config"
	pkgconfig "knative.dev/pkg/tracing/config"
)

const zipkinEndpointPattern = "(.*/api/v2/).*"

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
			"finish event should be received only once, received %d",
			f.received+1)
	}
	f.received++
	f.eventsSent = finished.EventsSent
	f.totalRequests = finished.TotalRequests
	log.Infof("finish event received, expecting %d event ware propagated", finished.EventsSent)
	d := config.Instance.Receiver.Teardown.Duration
	log.Infof("waiting additional %v to be sure all events came", d)
	time.Sleep(d)
	receivedEvents := f.steps.Count()
	if receivedEvents != finished.EventsSent {
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
		if unavailablePeriod > config.Instance.Receiver.Errors.UnavailablePeriodToReport {
			f.errors.throwUnavail("actual unavailable period %v is over down time limit of %v", unavailablePeriod, config.Instance.Receiver.Errors.UnavailablePeriodToReport)
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
	const eventFmt = "event #%v should be received once, but was received %v times"
	for eventNo := 1; eventNo <= finished.EventsSent; eventNo++ {
		times := steps.store[eventNo]
		if times == 0 {
			trace, err := GetTraceForEvent(eventNo)
			if err != nil {
				log.Warn(err)
			}
			f.errors.throwMissing(eventFmt+", trace:\n", eventNo, times, trace)
		} else if times > 1 {
			f.errors.throwDuplicated(eventFmt, eventNo, times)
		}
	}
}

func GetTraceForEvent(eventNo int) (string, error) {
	trace := "<unavailable>"
	cfg, err := pkgconfig.JSONToTracingConfig(config.Instance.TracingConfig)
	if err != nil {
		return trace, fmt.Errorf("failed to parse tracing config: %w", err)
	}
	if cfg.Backend != pkgconfig.None {
		r, _ := regexp.Compile(zipkinEndpointPattern)
		matches := r.FindStringSubmatch(cfg.ZipkinEndpoint)
		if len(matches) != 2 {
			return trace, fmt.Errorf("unsupported Zipkin endpoint %s: %w", cfg.ZipkinEndpoint, err)
		}
		tracesEndpoint := matches[1] + "traces"
		traceModel, err := FindEventTrace(tracesEndpoint, eventNo)
		if err != nil {
			return trace, fmt.Errorf("failed to find trace for event %d: %w", eventNo, err)
		}
		b, err := json.MarshalIndent(traceModel, "", "  ")
		if err != nil {
			return trace, fmt.Errorf("failed to unmarshall trace for event %d: %w", eventNo, err)
		}
		trace = string(b)
	}
	return trace, nil
}

func FindEventTrace(endpoint string, eventNo int) ([]model.SpanModel, error) {
	var empty []model.SpanModel

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return empty, err
	}
	req.Header.Add("spanName", "wathola-sender")
	req.Header.Add("annotationQuery",
		fmt.Sprintf("step=%d and cloudevents.type=%s", eventNo, StepType))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return empty, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return empty, err
	}

	var models [][]model.SpanModel
	err = json.Unmarshal(body, &models)
	if err != nil {
		return empty, fmt.Errorf("got an error in unmarshalling JSON %q: %w", body, err)
	}
	if len(models) != 1 {
		return empty, fmt.Errorf("found more than one trace for event: %d", len(models))
	}
	return models[0], nil
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
