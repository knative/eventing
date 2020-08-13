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

package recordevents

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	pkgTest "knative.dev/pkg/test"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
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
	tb testing.TB

	podName string
	getter  eventGetterInterface

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
func NewEventInfoStore(client *testlib.Client, podName string) (*EventInfoStore, error) {
	egi, err := newEventGetter(podName, client, client.T.Logf)
	if err != nil {
		return nil, err
	}
	ei := newTestableEventInfoStore(egi, -1, -1)
	ei.podName = podName
	ei.tb = client.T
	client.Cleanup(ei.cleanup)
	return ei, nil
}

type EventRecordOption = func(*corev1.Pod, *testlib.Client) error

// Deploys a new recordevents pod and start the associated EventInfoStore
func StartEventRecordOrFail(client *testlib.Client, podName string, options ...EventRecordOption) (*EventInfoStore, *corev1.Pod) {
	eventRecordPod := resources.EventRecordPod(podName)
	client.CreatePodOrFail(eventRecordPod, append(options, testlib.WithService(podName))...)
	err := pkgTest.WaitForPodRunning(client.Kube, podName, client.Namespace)
	if err != nil {
		client.T.Fatalf("Failed to start the recordevent pod '%s': %v", podName, errors.WithStack(err))
	}
	client.WaitForServiceEndpointsOrFail(podName, 1)

	eventTracker, err := NewEventInfoStore(client, podName)
	if err != nil {
		client.T.Fatalf("Failed to start the EventInfoStore associated to pod '%s': %v", podName, err)
	}
	return eventTracker, eventRecordPod
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
func (ei *EventInfoStore) cleanup() {
	close(ei.closeCh)
}

//TODO remove it, this is not useful anymore
// Deprecated: you can remove the manual cleanup of the event getter, since now it's done at test tear down automatically
func (ei *EventInfoStore) Cleanup() {}

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

// Find all events received by the recordevents pod that match the provided matchers,
// returning all matching events as well as a SearchedInfo structure including the
// last 5 events seen and the total events matched.  This SearchedInfo structure
// is primarily to ease debugging in failure printouts.  The provided function is
// guaranteed to be called exactly once on each EventInfo from the pod.
// The error array contains the eventual match errors, while the last return error contains
// an eventual communication error while trying to get the events from the recordevents pod
func (ei *EventInfoStore) Find(matchers ...EventInfoMatcher) ([]EventInfo, SearchedInfo, []error, error) {
	f := AllOf(matchers...)
	const maxLastEvents = 5
	allMatch := []EventInfo{}
	sInfo := SearchedInfo{}
	lastEvents := []EventInfo{}
	var nonMatchingErrors []error

	allEvents, err := ei.refreshData()
	if err != nil {
		return nil, sInfo, nonMatchingErrors, fmt.Errorf("error getting events %v", err)
	}
	for i := range allEvents {
		if err := f(allEvents[i]); err == nil {
			allMatch = append(allMatch, allEvents[i])
		} else {
			nonMatchingErrors = append(nonMatchingErrors, err)
		}
		lastEvents = append(lastEvents, allEvents[i])
		if len(lastEvents) > maxLastEvents {
			copy(lastEvents, lastEvents[1:])
			lastEvents = lastEvents[:maxLastEvents]
		}
	}
	sInfo.LastNEvent = lastEvents
	sInfo.TotalEvent = len(allEvents)

	return allMatch, sInfo, nonMatchingErrors, nil
}

// Assert that there are at least min number of match for the provided matchers.
// This method fails the test if the assert is not fulfilled.
func (ei *EventInfoStore) AssertAtLeast(min int, matchers ...EventInfoMatcher) []EventInfo {
	events, err := ei.waitAtLeastNMatch(AllOf(matchers...), min)
	if err != nil {
		ei.tb.Fatalf("Timeout waiting for at least %d matches.\nError: %+v", min, errors.WithStack(err))
	}
	return events
}

// Assert that there are at least min number of matches and at most max number of matches for the provided matchers.
// This method fails the test if the assert is not fulfilled.
func (ei *EventInfoStore) AssertInRange(min int, max int, matchers ...EventInfoMatcher) []EventInfo {
	events := ei.AssertAtLeast(min, matchers...)
	if max > 0 && len(events) > max {
		ei.tb.Fatalf("Assert in range failed: %+v", errors.WithStack(fmt.Errorf("expected <= %d events, saw %d", max, len(events))))
	}

	return events
}

// Assert that there aren't any matches for the provided matchers.
// This method fails the test if the assert is not fulfilled.
func (ei *EventInfoStore) AssertNot(matchers ...EventInfoMatcher) []EventInfo {
	res, recentEvents, _, err := ei.Find(matchers...)
	if err != nil {
		ei.tb.Fatalf("Unexpected error during find on recordevents '%s': %+v", ei.podName, errors.WithStack(err))
	}

	if len(res) != 0 {
		ei.tb.Fatalf("Assert not failed: %+v", errors.WithStack(
			fmt.Errorf("Unexpected matches on recordevents '%s', found: %v. %s", ei.podName, res, &recentEvents)),
		)
	}

	return res
}

// Assert that there are exactly n matches for the provided matchers.
// This method fails the test if the assert is not fulfilled.
func (ei *EventInfoStore) AssertExact(n int, matchers ...EventInfoMatcher) []EventInfo {
	return ei.AssertInRange(n, n, matchers...)
}

// Wait a long time (currently 4 minutes) until the provided function matches at least
// five events. The matching events are returned if we find at least n. If the
// function times out, an error is returned.
// If you need to perform assert on the result (aka you want to fail if error != nil), then use AssertAtLeast
func (ei *EventInfoStore) waitAtLeastNMatch(f EventInfoMatcher, min int) ([]EventInfo, error) {
	var matchRet []EventInfo
	var internalErr error

	wait.PollImmediate(ei.retryInterval, ei.timeout, func() (bool, error) {
		allMatch, sInfo, matchErrs, err := ei.Find(f)
		if err != nil {
			internalErr = fmt.Errorf("FAIL MATCHING: unexpected error during find: %v", err)
			return false, nil
		}
		count := len(allMatch)
		if count < min {
			internalErr = fmt.Errorf(
				"FAIL MATCHING: saw %d/%d matching events.\nRecent events: \n%s\nMatch errors: \n%s\n",
				count,
				min,
				&sInfo,
				formatErrors(matchErrs),
			)
			return false, nil
		}
		matchRet = allMatch
		internalErr = nil
		return true, nil
	})
	return matchRet, internalErr
}

func formatErrors(errs []error) string {
	var sb strings.Builder
	for i, err := range errs {
		sb.WriteString(strconv.Itoa(i) + " - ")
		sb.WriteString(err.Error())
		sb.WriteRune('\n')
	}
	return sb.String()
}
