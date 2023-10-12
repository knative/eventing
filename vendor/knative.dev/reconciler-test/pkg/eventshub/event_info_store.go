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

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/logging"

	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
)

// EventInfoMatcher returns an error if the input event info doesn't match the criteria
type EventInfoMatcher func(EventInfo) error

// WithContext transforms EventInfoMatcher to EventInfoMatcherCtx.
func (m EventInfoMatcher) WithContext() EventInfoMatcherCtx {
	return func(ctx context.Context, info EventInfo) error {
		return m(info)
	}
}

// EventInfoMatcherCtx returns an error if the input event info doesn't match the criteria
type EventInfoMatcherCtx func(context.Context, EventInfo) error

// WithContext transforms EventInfoMatcherCtx to EventInfoMatcher.
func (m EventInfoMatcherCtx) WithContext(ctx context.Context) EventInfoMatcher {
	return func(info EventInfo) error {
		return m(ctx, info)
	}
}

// Stateful store of events published by eventshub pod it is pointed at.
// Implements k8s.EventHandler
type Store struct {
	podName      string
	podNamespace string

	lock      sync.Mutex
	collected []EventInfo

	eventsSeen    int
	eventsNotMine int
}

func StoreFromContext(ctx context.Context, name string) *Store {
	if store, ok := k8s.EventListenerFromContext(ctx).GetHandler(name).(*Store); ok && store != nil {
		return store
	}
	panic("no event store found in the context for the provided name " + name)
}

func registerEventsHubStore(ctx context.Context, eventListener *k8s.EventListener, podName string, podNamespace string) {
	store := &Store{
		podName:      podName,
		podNamespace: podNamespace,
	}

	numEventsAlreadyPresent := eventListener.AddHandler(podName, store)
	logging.FromContext(ctx).
		Infof("Store added to the EventListener, which has already seen %v events",
			numEventsAlreadyPresent)
}

func (ei *Store) getDebugInfo() string {
	return fmt.Sprintf("Pod '%s' in namespace '%s'", ei.podName, ei.podNamespace)
}

func (ei *Store) Collected() []EventInfo {
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

	eventInfo := EventInfo{}
	err := json.Unmarshal([]byte(event.Message), &eventInfo)
	if err != nil {
		fmt.Printf("[ERROR] Received EventInfo that cannot be unmarshalled! \n----\n%s\n----\n%+v\n", event.Message, err)
		return
	}

	ei.collected = append(ei.collected, eventInfo)
}

func (ei *Store) isMyEvent(event *corev1.Event) bool {
	return event.Type == corev1.EventTypeNormal &&
		event.Reason == CloudEventObservedReason &&
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
func (ei *Store) Find(matchers ...EventInfoMatcher) ([]EventInfo, SearchedInfo, []error, error) {
	f := allOf(matchers...)
	const maxLastEvents = 5
	allMatch := []EventInfo{}
	ei.lock.Lock()
	sInfo := SearchedInfo{
		StoreEventsSeen:    ei.eventsSeen,
		StoreEventsNotMine: ei.eventsNotMine,
	}
	ei.lock.Unlock()
	lastEvents := []EventInfo{}
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
func (ei *Store) AssertAtLeast(ctx context.Context, t feature.T, min int, matchers ...EventInfoMatcher) []EventInfo {
	events, err := ei.waitAtLeastNMatch(ctx, allOf(matchers...), min)
	if err != nil {
		t.Fatalf("Timeout waiting for at least %d matches.\nError: %+v", min, errors.WithStack(err))
	}
	return events
}

// AssertInRange asserts that there are at least min number of matches and at most max number of matches for the provided matchers.
// This method fails the test if the assert is not fulfilled.
func (ei *Store) AssertInRange(ctx context.Context, t feature.T, min int, max int, matchers ...EventInfoMatcher) []EventInfo {
	events := ei.AssertAtLeast(ctx, t, min, matchers...)
	if max > 0 && len(events) > max {
		t.Fatalf("Assert in range failed: %+v", errors.WithStack(fmt.Errorf("expected <= %d events, saw %d", max, len(events))))
	}
	return events
}

// AssertNot asserts that there aren't any matches for the provided matchers.
// This method fails the test if the assert is not fulfilled.
func (ei *Store) AssertNot(t feature.T, matchers ...EventInfoMatcher) []EventInfo {
	res, recentEvents, _, err := ei.Find(matchers...)
	if err != nil {
		t.Fatalf("Unexpected error during find on eventshub '%s': %+v", ei.podName, errors.WithStack(err))
	}

	if len(res) != 0 {
		t.Fatalf("Assert not failed: %+v", errors.WithStack(
			fmt.Errorf("Unexpected matches on eventshub '%s', found: %v. %s", ei.podName, res, &recentEvents)),
		)
	}
	return res
}

// AssertExact assert that there are exactly n matches for the provided matchers.
// This method fails the test if the assert is not fulfilled.
func (ei *Store) AssertExact(ctx context.Context, t feature.T, n int, matchers ...EventInfoMatcher) []EventInfo {
	events := ei.AssertInRange(ctx, t, n, n, matchers...)
	return events
}

// Wait a long time (currently 4 minutes) until the provided function matches at least
// five events. The matching events are returned if we find at least n. If the
// function times out, an error is returned.
// If you need to perform assert on the result (aka you want to fail if error != nil), then use AssertAtLeast
func (ei *Store) waitAtLeastNMatch(ctx context.Context, f EventInfoMatcher, min int) ([]EventInfo, error) {
	var matchRet []EventInfo
	var internalErr error

	retryInterval, retryTimeout := environment.PollTimingsFromContext(ctx)

	wait.PollImmediate(retryInterval, retryTimeout, func() (bool, error) {
		allMatch, sInfo, matchErrs, err := ei.Find(f)
		if err != nil {
			internalErr = fmt.Errorf("FAIL MATCHING: unexpected error during find: %v", err)
			return false, nil
		}
		count := len(allMatch)
		if count < min {
			internalErr = fmt.Errorf(
				"FAIL MATCHING: saw %d/%d matching events.\n- Store-\n%s\n- Recent events -\n%s\n- Match errors -\n%s\nCollected events: %s",
				count,
				min,
				ei.getDebugInfo(),
				&sInfo,
				formatErrors(matchErrs),
				ei.dumpCollected(),
			)
			return false, nil
		}
		matchRet = allMatch
		internalErr = nil
		return true, nil
	})
	return matchRet, internalErr
}

func (ei *Store) dumpCollected() string {
	var sb strings.Builder
	for _, e := range ei.Collected() {
		sb.WriteString(e.String())
		sb.WriteRune('\n')
	}
	return sb.String()
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
	return func(have EventInfo) error {
		for _, m := range matchers {
			if err := m(have); err != nil {
				return err
			}
		}
		return nil
	}
}
