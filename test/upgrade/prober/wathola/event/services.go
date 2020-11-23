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

var mutex = sync.RWMutex{}
var lastProgressReport = time.Now()

const (
	TagDuplicated = "DUPLICATE"
	TagDefault    = "DEFAULT"
	TagMissing    = "MISSING"
)

// ErrorStore contains errors that was thrown
type ErrorStore struct {
	state            State
	missingThrown    []thrown
	duplicatedThrown []thrown
	defaultThrown    []thrown
}

// NewErrorStore creates a new error store
func NewErrorStore() *ErrorStore {
	return &ErrorStore{
		state:            Active,
		missingThrown:    make([]thrown, 0),
		duplicatedThrown: make([]thrown, 0),
		defaultThrown:    make([]thrown, 0),
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
		received: 0,
		count:    -1,
		steps:    steps,
		errors:   errors,
	}
}

func (s *stepStore) RegisterStep(step *Step) {
	mutex.Lock()
	if times, found := s.store[step.Number]; found {
		s.errors.throw(TagDuplicated,
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
		f.errors.throw(TagDuplicated,
			"finish event should be received only once, received %d",
			f.received+1)
	}
	f.received++
	f.count = finished.Count
	log.Infof("finish event received, expecting %d event ware propagated", finished.Count)
	d := config.Instance.Receiver.Teardown.Duration
	log.Infof("waiting additional %v to be sure all events came", d)
	time.Sleep(d)
	receivedEvents := f.steps.Count()
	if receivedEvents != finished.Count {
		var tag string
		if receivedEvents < finished.Count {
			tag = TagMissing
		} else {
			tag = TagDefault
		}
		f.errors.throw(tag, "expecting to have %v unique events received, "+
			"but received %v unique events", finished.Count, receivedEvents)
		f.reportViolations(finished)
		f.errors.state = Failed
	} else {
		log.Infof("properly received %d unique events", receivedEvents)
		f.errors.state = Success
	}
}

func (f *finishedStore) State() State {
	return f.errors.state
}

func (f *finishedStore) DuplicatedThrown() []string {
	return f.thrownHelper(TagDuplicated)
}

func (f *finishedStore) MissingThrown() []string {
	return f.thrownHelper(TagMissing)
}

func (f *finishedStore) DefaultThrown() []string {
	return f.thrownHelper(TagDefault)
}

func (f *finishedStore) thrownHelper(tag string) []string {
	msgs := make([]string, 0)
	thrownChannel := f.errors.defaultThrown
	if tag == TagDuplicated {
		thrownChannel = f.errors.duplicatedThrown
	} else if tag == TagMissing {
		thrownChannel = f.errors.missingThrown
	}
	for _, t := range thrownChannel {
		errMsg := fmt.Sprintf(t.format, t.args...)
		msgs = append(msgs, errMsg)
	}
	return msgs
}

func (f *finishedStore) reportViolations(finished *Finished) {
	steps := f.steps.(*stepStore)
	for eventNo := 1; eventNo <= finished.Count; eventNo++ {
		times, ok := steps.store[eventNo]
		if !ok {
			times = 0
		}
		if times != 1 {
			var tag string
			if times == 0 {
				tag = TagMissing
			} else {
				tag = TagDuplicated
			}
			f.errors.throw(tag, "event #%v should be received once, but was received %v times",
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

func (e *ErrorStore) throw(tag string, format string, args ...interface{}) {
	t := thrown{
		format: format,
		args:   args,
	}
	if tag == TagDuplicated {
		e.duplicatedThrown = append(e.duplicatedThrown, t)
	} else if tag == TagMissing {
		e.missingThrown = append(e.missingThrown, t)
	} else {
		e.defaultThrown = append(e.defaultThrown, t)
	}
	log.Errorf(t.format, t.args...)
}

type stepStore struct {
	store  map[int]int
	errors *ErrorStore
}

type finishedStore struct {
	received int
	count    int
	errors   *ErrorStore
	steps    StepsStore
}

type thrown struct {
	format string
	args   []interface{}
}
