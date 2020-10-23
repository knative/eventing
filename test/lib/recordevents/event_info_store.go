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
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	testlib "knative.dev/eventing/test/lib"
)

const (
	// The interval and timeout used for checking events
	retryInterval = 4 * time.Second
	retryTimeout  = 4 * time.Minute
)

// Deploys a new recordevents pod and start the associated EventInfoStore
func StartEventRecordOrFail(ctx context.Context, client *testlib.Client, podName string, options ...EventRecordOption) (*EventInfoStore, *corev1.Pod) {
	eventRecordPod := DeployEventRecordOrFail(ctx, client, podName, options...)

	eventTracker, err := NewEventInfoStore(client, podName, client.Namespace)
	if err != nil {
		client.T.Fatalf("Failed to start the EventInfoStore associated to pod '%s': %v", podName, err)
	}
	return eventTracker, eventRecordPod
}

// Stateful store of events received by the recordevents pod it is pointed at.
// This pulls events from the pod during any Find or Wait call, storing them
// locally and trimming them from the remote pod store.
type EventInfoStore struct {
	tb testing.TB

	podName      string
	podNamespace string

	lock      sync.Mutex
	collected []EventInfo

	eventsSeen    int
	eventsNotMine int
}

// Creates an EventInfoStore that is used to iteratively download events recorded by the
// recordevents pod.
func NewEventInfoStore(client *testlib.Client, podName string, podNamespace string) (*EventInfoStore, error) {
	store := &EventInfoStore{
		tb:           client.T,
		podName:      podName,
		podNamespace: podNamespace,
	}

	numEventsAlreadyPresent := client.EventListener.AddHandler(store.handle)
	client.T.Logf("EventInfoStore added to the EventListener, which has already seen %v events", numEventsAlreadyPresent)

	return store, nil
}

func (ei *EventInfoStore) getEventInfo() []EventInfo {
	ei.lock.Lock()
	defer ei.lock.Unlock()
	return ei.collected
}

func (ei *EventInfoStore) handle(event *corev1.Event) {
	ei.lock.Lock()
	defer ei.lock.Unlock()
	ei.eventsSeen += 1
	// Filter events
	if !ei.isMyEvent(event) {
		ei.eventsNotMine += 1
		return
	}

	eventInfo := EventInfo{}
	err := json.Unmarshal([]byte(event.Message), &eventInfo)
	if err != nil {
		ei.tb.Errorf("Received EventInfo that cannot be unmarshalled! %+v", err)
		return
	}

	ei.collected = append(ei.collected, eventInfo)
}

func (ei *EventInfoStore) isMyEvent(event *corev1.Event) bool {
	return event.Type == corev1.EventTypeNormal &&
		event.Reason == CloudEventObservedReason &&
		event.InvolvedObject.Kind == "Pod" &&
		event.InvolvedObject.Name == ei.podName &&
		event.InvolvedObject.Namespace == ei.podNamespace
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
	ei.lock.Lock()
	sInfo := SearchedInfo{
		storeEventsSeen:    ei.eventsSeen,
		storeEventsNotMine: ei.eventsNotMine,
	}
	ei.lock.Unlock()
	lastEvents := []EventInfo{}
	var nonMatchingErrors []error

	allEvents := ei.getEventInfo()
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
	ei.tb.Helper()
	events, err := ei.waitAtLeastNMatch(AllOf(matchers...), min)
	if err != nil {
		ei.tb.Fatalf("Timeout waiting for at least %d matches.\nError: %+v", min, errors.WithStack(err))
	}
	ei.tb.Logf("Assert passed")
	return events
}

// Assert that there are at least min number of matches and at most max number of matches for the provided matchers.
// This method fails the test if the assert is not fulfilled.
func (ei *EventInfoStore) AssertInRange(min int, max int, matchers ...EventInfoMatcher) []EventInfo {
	ei.tb.Helper()
	events := ei.AssertAtLeast(min, matchers...)
	if max > 0 && len(events) > max {
		ei.tb.Fatalf("Assert in range failed: %+v", errors.WithStack(fmt.Errorf("expected <= %d events, saw %d", max, len(events))))
	}
	ei.tb.Logf("Assert passed")
	return events
}

// Assert that there aren't any matches for the provided matchers.
// This method fails the test if the assert is not fulfilled.
func (ei *EventInfoStore) AssertNot(matchers ...EventInfoMatcher) []EventInfo {
	ei.tb.Helper()
	res, recentEvents, _, err := ei.Find(matchers...)
	if err != nil {
		ei.tb.Fatalf("Unexpected error during find on recordevents '%s': %+v", ei.podName, errors.WithStack(err))
	}

	if len(res) != 0 {
		ei.tb.Fatalf("Assert not failed: %+v", errors.WithStack(
			fmt.Errorf("Unexpected matches on recordevents '%s', found: %v. %s", ei.podName, res, &recentEvents)),
		)
	}
	ei.tb.Logf("Assert passed")
	return res
}

// Assert that there are exactly n matches for the provided matchers.
// This method fails the test if the assert is not fulfilled.
func (ei *EventInfoStore) AssertExact(n int, matchers ...EventInfoMatcher) []EventInfo {
	ei.tb.Helper()
	events := ei.AssertInRange(n, n, matchers...)
	ei.tb.Logf("Assert passed")
	return events
}

// Wait a long time (currently 4 minutes) until the provided function matches at least
// five events. The matching events are returned if we find at least n. If the
// function times out, an error is returned.
// If you need to perform assert on the result (aka you want to fail if error != nil), then use AssertAtLeast
func (ei *EventInfoStore) waitAtLeastNMatch(f EventInfoMatcher, min int) ([]EventInfo, error) {
	var matchRet []EventInfo
	var internalErr error

	wait.PollImmediate(retryInterval, retryTimeout, func() (bool, error) {
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
