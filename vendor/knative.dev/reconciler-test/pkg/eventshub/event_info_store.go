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

package eventshub

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/test_images/eventshub"
)

const (
	// The interval and timeout used for checking events
	retryInterval = 4 * time.Second
	retryTimeout  = 4 * time.Minute
)

// EventInfoMatcher returns an error if the input event info doesn't match the criteria
type EventInfoMatcher func(eventshub.EventInfo) error

// Stateful store of events published by eventshub pod it is pointed at.
// Implements k8s.EventHandler
type Store struct {
	t feature.T

	podName      string
	podNamespace string

	lock      sync.Mutex
	collected []eventshub.EventInfo

	eventsSeen    int
	eventsNotMine int
}

func StoreFromContext(ctx context.Context, name string) *Store {
	if store, ok := k8s.EventListenerFromContext(ctx).GetHandler(name).(*Store); ok && store != nil {
		return store
	}
	panic("no event store found in the context for the provided name " + name)
}

func registerEventsHubStore(eventListener *k8s.EventListener, t feature.T, podName string, podNamespace string) {
	store := &Store{
		t:            t,
		podName:      podName,
		podNamespace: podNamespace,
	}

	numEventsAlreadyPresent := eventListener.AddHandler(podName, store)
	t.Logf("Store added to the EventListener, which has already seen %v events", numEventsAlreadyPresent)
}

func (ei *Store) getDebugInfo() string {
	return fmt.Sprintf("Pod '%s' in namespace '%s'", ei.podName, ei.podNamespace)
}

func (ei *Store) Collected() []eventshub.EventInfo {
	ei.lock.Lock()
	defer ei.lock.Unlock()
	return ei.collected
}

func (ei *Store) Handle(event *corev1.Event) {
	ei.lock.Lock()
	defer ei.lock.Unlock()
	ei.eventsSeen += 1
	// Filter events
	if !ei.isMyEvent(event) {
		ei.eventsNotMine += 1
		return
	}

	eventInfo := eventshub.EventInfo{}
	err := json.Unmarshal([]byte(event.Message), &eventInfo)
	if err != nil {
		ei.t.Errorf("Received EventInfo that cannot be unmarshalled! %+v", err)
		return
	}

	ei.collected = append(ei.collected, eventInfo)
}

func (ei *Store) isMyEvent(event *corev1.Event) bool {
	return event.Type == corev1.EventTypeNormal &&
		event.Reason == eventshub.CloudEventObservedReason &&
		event.InvolvedObject.Kind == "Pod" &&
		event.InvolvedObject.Name == ei.podName &&
		event.InvolvedObject.Namespace == ei.podNamespace
}

// Find all events received by the eventshub pod that match the provided matchers,
// returning all matching events as well as a SearchedInfo structure including the
// last 5 events seen and the total events matched.  This SearchedInfo structure
// is primarily to ease debugging in failure printouts.  The provided function is
// guaranteed to be called exactly once on each EventInfo from the pod.
// The error array contains the eventual match errors, while the last return error contains
// an eventual communication error while trying to get the events from the eventshub pod
func (ei *Store) Find(matchers ...EventInfoMatcher) ([]eventshub.EventInfo, eventshub.SearchedInfo, []error, error) {
	f := allOf(matchers...)
	const maxLastEvents = 5
	allMatch := []eventshub.EventInfo{}
	ei.lock.Lock()
	sInfo := eventshub.SearchedInfo{
		StoreEventsSeen:    ei.eventsSeen,
		StoreEventsNotMine: ei.eventsNotMine,
	}
	ei.lock.Unlock()
	lastEvents := []eventshub.EventInfo{}
	var nonMatchingErrors []error

	allEvents := ei.Collected()
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

// AssertAtLeast assert that there are at least min number of match for the provided matchers.
// This method fails the test if the assert is not fulfilled.
func (ei *Store) AssertAtLeast(min int, matchers ...EventInfoMatcher) []eventshub.EventInfo {
	events, err := ei.waitAtLeastNMatch(allOf(matchers...), min)
	if err != nil {
		ei.t.Fatalf("Timeout waiting for at least %d matches.\nError: %+v", min, errors.WithStack(err))
	}
	ei.t.Logf("Assert passed")
	return events
}

// AssertInRange asserts that there are at least min number of matches and at most max number of matches for the provided matchers.
// This method fails the test if the assert is not fulfilled.
func (ei *Store) AssertInRange(min int, max int, matchers ...EventInfoMatcher) []eventshub.EventInfo {
	events := ei.AssertAtLeast(min, matchers...)
	if max > 0 && len(events) > max {
		ei.t.Fatalf("Assert in range failed: %+v", errors.WithStack(fmt.Errorf("expected <= %d events, saw %d", max, len(events))))
	}
	ei.t.Logf("Assert passed")
	return events
}

// AssertNot asserts that there aren't any matches for the provided matchers.
// This method fails the test if the assert is not fulfilled.
func (ei *Store) AssertNot(matchers ...EventInfoMatcher) []eventshub.EventInfo {
	res, recentEvents, _, err := ei.Find(matchers...)
	if err != nil {
		ei.t.Fatalf("Unexpected error during find on eventshub '%s': %+v", ei.podName, errors.WithStack(err))
	}

	if len(res) != 0 {
		ei.t.Fatalf("Assert not failed: %+v", errors.WithStack(
			fmt.Errorf("Unexpected matches on eventshub '%s', found: %v. %s", ei.podName, res, &recentEvents)),
		)
	}
	ei.t.Logf("Assert passed")
	return res
}

// AssertExact assert that there are exactly n matches for the provided matchers.
// This method fails the test if the assert is not fulfilled.
func (ei *Store) AssertExact(n int, matchers ...EventInfoMatcher) []eventshub.EventInfo {
	events := ei.AssertInRange(n, n, matchers...)
	ei.t.Logf("Assert passed")
	return events
}

// Wait a long time (currently 4 minutes) until the provided function matches at least
// five events. The matching events are returned if we find at least n. If the
// function times out, an error is returned.
// If you need to perform assert on the result (aka you want to fail if error != nil), then use AssertAtLeast
func (ei *Store) waitAtLeastNMatch(f EventInfoMatcher, min int) ([]eventshub.EventInfo, error) {
	var matchRet []eventshub.EventInfo
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
				"FAIL MATCHING: saw %d/%d matching events.\n- Store-\n%s\n- Recent events -\n%s\n- Match errors -\n%s\n",
				count,
				min,
				ei.getDebugInfo(),
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

// We don't need to expose this, since all the signatures already executes this
func allOf(matchers ...EventInfoMatcher) EventInfoMatcher {
	return func(have eventshub.EventInfo) error {
		for _, m := range matchers {
			if err := m(have); err != nil {
				return err
			}
		}
		return nil
	}
}
