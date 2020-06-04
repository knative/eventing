/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package lib

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"k8s.io/apimachinery/pkg/util/wait"

	"knative.dev/pkg/test/logging"
)

const (
	// The interval and timeout used for checking events
	minEvRetryInterval = 4 * time.Second
	timeoutEvRetry     = 4 * time.Minute
)

// Stateful store of events received by the recordevents pod it is pointed at.
// This pulls events from the pod during any Find or Wait call, storing them
// locally and triming them from the remote pod store.
type EventInfoStore struct {
	getter eventGetterInterface

	lock          sync.Mutex
	allEvents     []EventInfo
	firstID       int
	closeCh       chan struct{}
	doRefresh     chan chan error
	timeout       time.Duration
	retryInterval time.Duration
}

// Functions used for getting data from the REST api of the recordevents pod.
// The interface exists for use with unit tests of this module.
type eventGetterInterface interface {
	getMinMax() (minRet int, maxRet int, errRet error)
	getEntry(seqno int) (EventInfo, error)
	trimThrough(seqno int) error
	cleanup()
}

// Internal function to create an event store.  This is called directly by unit tests of
// this module.
func newTestableEventInfoStore(egi eventGetterInterface, retryInterval time.Duration,
	timeout time.Duration) *EventInfoStore {
	if timeout == -1 {
		timeout = timeoutEvRetry
	}
	if retryInterval == -1 {
		retryInterval = minEvRetryInterval
	}
	ei := &EventInfoStore{getter: egi, firstID: 1, timeout: timeout, retryInterval: retryInterval}
	ei.start()
	return ei
}

// Creates an EventInfoStore that is used to iteratively download events recorded by the
// recordevents pod.  Calling this forwards the recordevents port to the local machine
// and blocks waiting to connect to that pod.  Fails if it cannot connect within
// the expected timeout (4 minutes currently)
func (c *Client) NewEventInfoStore(podName string, logf logging.FormatLogger) (*EventInfoStore, error) {
	egi, err := newEventGetter(podName, c, logf)
	if err != nil {
		return nil, err
	}
	ei := newTestableEventInfoStore(egi, -1, -1)
	return ei, nil
}

// Starts the single threaded background goroutine used to update local state
// from the remote REST API.
func (ei *EventInfoStore) start() {
	ei.closeCh = make(chan struct{})
	ei.doRefresh = make(chan chan error)
	go func() {
		for {
			select {
			case <-ei.closeCh:
				ei.getter.cleanup()
				return
			case replyCh := <-ei.doRefresh:
				replyCh <- ei.doRetrieveData()
			}
		}
	}()
}

// The data update thread used by the single threaded background goroutine
// for updating data from the REST api.
func (ei *EventInfoStore) doRetrieveData() error {
	min, max, err := ei.getter.getMinMax()
	if err != nil {
		return fmt.Errorf("error getting MinMax %v", err)
	}
	ei.lock.Lock()
	curMin := ei.firstID
	curMax := curMin + len(ei.allEvents) - 1
	ei.lock.Unlock()
	if min == max+1 {
		// Nothing to read or trim
		return nil
	} else {
		if min > curMax+1 {
			return fmt.Errorf("mismatched stored max/available min: %d, %d", curMax, min)
		}
		min = curMax + 1
		// We may have data to read, definitely have data to trim.
	}
	var newEvents []EventInfo
	for i := min; i <= max; i++ {
		e, err := ei.getter.getEntry(i)
		if err != nil {
			return fmt.Errorf("error calling getEntry of %d %v", i, err)
		}

		newEvents = append(newEvents, e)
	}
	ei.lock.Lock()
	ei.allEvents = append(ei.allEvents, newEvents...)
	ei.lock.Unlock()
	err = ei.getter.trimThrough(max)
	return err

}

// Clean up any background resources used by the store.  Must be called exactly once after
// the last use.
func (ei *EventInfoStore) Cleanup() {
	close(ei.closeCh)
}

// Called internally by functions wanting the current list of all
// known events.  This calls for an update from the REST server and
// returns the summary of all locally and remotely known events.
// Returns an error in case of a connection or protocol error.
func (ei *EventInfoStore) refreshData() ([]EventInfo, error) {
	var allEvents []EventInfo
	replyCh := make(chan error)
	ei.doRefresh <- replyCh
	err := <-replyCh
	if err != nil {
		return nil, err
	}
	ei.lock.Lock()
	allEvents = append(allEvents, ei.allEvents...)
	ei.lock.Unlock()
	return allEvents, nil
}

// Find all events received by the recordevents pod that match the provided function,
// returning all matching events as well as a SearchedInfo structure including the
// last 5 events seen and the total events matched.  This SearchedInfo structure
// is primarily to ease debugging in failure printouts.  The provided function is
// guaranteed to be called exactly once on each EventInfo from the pod.
func (ei *EventInfoStore) Find(f EventInfoMatchFunc) ([]EventInfo, SearchedInfo, error) {
	const maxLastEvents = 5
	allMatch := []EventInfo{}
	sInfo := SearchedInfo{}
	lastEvents := []EventInfo{}

	allEvents, err := ei.refreshData()
	if err != nil {
		return nil, sInfo, fmt.Errorf("error getting events %v", err)
	}
	for i := range allEvents {
		if f(allEvents[i]) == nil {
			allMatch = append(allMatch, allEvents[i])
		}
		lastEvents = append(lastEvents, allEvents[i])
		if len(lastEvents) > maxLastEvents {
			copy(lastEvents, lastEvents[1:])
			lastEvents = lastEvents[:maxLastEvents]
		}
	}
	sInfo.LastNEvent = lastEvents
	sInfo.TotalEvent = len(allEvents)

	return allMatch, sInfo, nil
}

// Convert a boolean check function that checks valid messages to a function
// that checks EventInfo structures, returning false for any that don't
// contain valid events.
func ValidEvFunc(evf EventMatchFunc) EventInfoMatchFunc {
	return func(ei EventInfo) error {
		if ei.Event == nil {
			return fmt.Errorf("Saw nil event")
		} else {
			return evf(*ei.Event)
		}
	}
}

// Wait a long time (currently 4 minutes) until the provided function matches at least
// five events.  The matching events are returned if we find at least n.  If the
// function times out, an error is returned.
func (ei *EventInfoStore) WaitAtLeastNMatch(f EventInfoMatchFunc, n int) ([]EventInfo, error) {
	var matchRet []EventInfo
	var internalErr error

	wait.PollImmediate(ei.retryInterval, ei.timeout, func() (bool, error) {
		allMatch, sInfo, err := ei.Find(f)
		if err != nil {
			internalErr = fmt.Errorf("FAIL MATCHING: unexpected error during find: %v", err)
			return false, nil
		}
		count := len(allMatch)
		if count < n {
			internalErr = fmt.Errorf("FAIL MATCHING: saw %d/%d matching events. recent events: (%s)",
				count, n, &sInfo)
			return false, nil
		}
		matchRet = allMatch
		internalErr = nil
		return true, nil
	})
	return matchRet, internalErr
}

func (ei *EventInfoStore) MustWaitAtLeastNMatch(t testing.TB, f EventInfoMatchFunc, n int) []EventInfo {
	events, err := ei.WaitAtLeastNMatch(f, n)
	if err != nil {
		t.Fatalf("Timeout waiting for %d matches. Error: %v", n, err)
	}

	return events
}

// Wait for at least minCount events with source exactly matching source and data contained within the event
// data field.  If source is the empty string, don't check the source.  If maxCount is >0, return an error
// if more than maxCount entries are seen.
func (ei *EventInfoStore) WaitMatchSourceData(source string, data string, minCount int, maxCount int) error {
	matchFunc := func(ev cloudevents.Event) error {
		if source != "" && ev.Source() != source {
			return fmt.Errorf("mismatched source: expected %s, saw %s", source, ev.Source())
		}
		db := ev.Data()
		body := string(db)
		if strings.Contains(body, data) {
			return nil
		} else {
			return fmt.Errorf("didn't find substring (%s) in data (%s)", data, body)
		}
	}
	// verify the logger service receives the event and only once
	match, err := ei.WaitAtLeastNMatch(ValidEvFunc(matchFunc), minCount)
	if err != nil {
		return fmt.Errorf("error waiting for event: %v", err)
	}
	if maxCount > 0 && len(match) > maxCount {
		return fmt.Errorf("expected <= %d events, saw %d", maxCount, len(match))
	}
	return nil
}

func (ei *EventInfoStore) AssertWaitMatchSourceData(tb testing.TB, eventRecord string, source string, data string, minCount int, maxCount int) {
	if err := ei.WaitMatchSourceData(source, data, minCount, maxCount); err != nil {
		tb.Fatalf("Timeout waiting for source %q and data %q. It does not appear at least %d times in the event record pod %q: %v", source, data, minCount, eventRecord, err)
	}
}

// Does the provided EventInfo match some criteria
type EventInfoMatchFunc func(EventInfo) error

// Does the provided event match some criteria
type EventMatchFunc func(cloudevents.Event) error

func MatchAllEvent(cloudevents.Event) error {
	return nil
}
