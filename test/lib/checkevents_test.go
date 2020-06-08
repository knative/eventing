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
	"strconv"
	"sync"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type dummyEventGet struct {
	lock sync.Mutex

	minSeq int
	allEv  []EventInfo
	clean  bool
	tCalls int
}

func newDummyEventGet() *dummyEventGet {
	return &dummyEventGet{minSeq: 1}
}

func (deg *dummyEventGet) getMinMax() (minRet int, maxRet int, errRet error) {
	deg.lock.Lock()
	defer deg.lock.Unlock()
	if deg.clean {
		panic("getminmax called on cleaned event getter")
	}
	min := deg.minSeq
	max := min + len(deg.allEv) - 1
	return min, max, nil
}
func (deg *dummyEventGet) getEntry(seqno int) (EventInfo, error) {
	deg.lock.Lock()
	defer deg.lock.Unlock()
	if deg.clean {
		panic("getentry called on cleaned event getter")
	}
	min := deg.minSeq
	max := min + len(deg.allEv) - 1
	if seqno >= min && seqno <= max {
		return deg.allEv[seqno-deg.minSeq], nil
	} else {
		return EventInfo{}, fmt.Errorf("illegal get seqno: %d, range min = %d, max = %d", seqno, min, max)
	}
}
func (deg *dummyEventGet) trimThrough(seqno int) error {
	deg.lock.Lock()
	defer deg.lock.Unlock()
	deg.tCalls++
	if deg.clean {
		panic("trim called on cleaned event getter")
	}
	min := deg.minSeq
	max := min + len(deg.allEv) - 1
	if seqno < 0 {
		return fmt.Errorf("Illegal negative seqno %d", seqno)
	} else if seqno > max {
		return fmt.Errorf("Illegal negative seqno %d > %d", seqno, max)
	} else if seqno < min {
		return nil
	}
	deg.allEv = deg.allEv[seqno-min+1:]
	deg.minSeq = seqno + 1
	return nil
}
func (deg *dummyEventGet) trimCalls() int {
	deg.lock.Lock()
	defer deg.lock.Unlock()
	return deg.tCalls
}
func (deg *dummyEventGet) cleanup() {
	deg.lock.Lock()
	defer deg.lock.Unlock()
	if deg.clean {
		panic("Unexpected clean")
	} else {
		deg.clean = true
	}
}

func makeEvents() []EventInfo {
	var allEv []EventInfo
	for i := 0; i < 30; i++ {
		ce := cloudevents.NewEvent(cloudevents.VersionV1)
		ce.SetType("knative.dev.test.event.a")
		ce.SetSource("https://source.test.event.knative.dev/foo")
		ce.SetID(strconv.FormatInt(int64(i), 10))
		allEv = append(allEv, EventInfo{Event: &ce})
	}
	return allEv
}

func checkEvIDEqual(t *testing.T, seen []EventInfo, expected []EventInfo) {
	if len(seen) != len(expected) {
		t.Fatalf("Seen ev list length %d does not match expected %d", len(seen), len(expected))

	}
	for i := range seen {
		if seen[i].Event.ID() != expected[i].Event.ID() {
			t.Fatalf("Id entry %d mismatch: seen %s != expected %s",
				i, seen[i].Event.ID(), expected[i].Event.ID())
		}
	}
}

func (deg *dummyEventGet) setEv(firstID int, allEv []EventInfo) {
	deg.lock.Lock()
	defer deg.lock.Unlock()
	deg.minSeq = firstID
	deg.allEv = allEv
}

// Test that Finds where the server gives sequential updates that
// don't overlap see the expected events.
func TestSequentialAndTrim(t *testing.T) {
	totalEv := makeEvents()
	expectedFull := makeEvents()
	deg := newDummyEventGet()
	subEv := totalEv[:10]
	deg.setEv(1, subEv)
	ei := newTestableEventInfoStore(deg, -1, -1)
	allData, _, err := ei.Find(func(EventInfo) error { return nil })
	if err != nil {
		t.Fatalf("Unexpected error from find: %v", err)
	}
	checkEvIDEqual(t, allData, expectedFull[:10])
	min, _, _ := deg.getMinMax()
	if min != 11 {
		t.Fatalf("Expected trim to have moved min to %d, saw %d", 11, min)
	}

	subEv = totalEv[10:19]
	deg.setEv(11, subEv)

	allData, _, err = ei.Find(func(EventInfo) error { return nil })
	if err != nil {
		t.Fatalf("Unexpected error from find: %v", err)
	}
	checkEvIDEqual(t, allData, expectedFull[:19])
	ei.Cleanup()
}

// Test that Finds where the server gives overlapping updates
// see the expected events (no double events)
func TestOverlap(t *testing.T) {
	totalEv := makeEvents()
	expectedFull := makeEvents()
	deg := newDummyEventGet()
	subEv := totalEv[:10]
	deg.setEv(1, subEv)
	ei := newTestableEventInfoStore(deg, -1, -1)
	allData, _, err := ei.Find(func(EventInfo) error { return nil })
	if err != nil {
		t.Fatalf("Unexpected error from find: %v", err)
	}
	checkEvIDEqual(t, allData, expectedFull[:10])
	subEv = totalEv[6:19]
	deg.setEv(7, subEv)
	allData, _, err = ei.Find(func(EventInfo) error { return nil })
	if err != nil {
		t.Fatalf("Unexpected error from find: %v", err)
	}
	checkEvIDEqual(t, allData, expectedFull[:19])
	ei.Cleanup()
}

// Test that we see an error if repeated Finds see a gap in the sequence
// space
func TestGap(t *testing.T) {
	totalEv := makeEvents()
	expectedFull := makeEvents()
	deg := newDummyEventGet()
	subEv := totalEv[:10]
	deg.setEv(1, subEv)
	ei := newTestableEventInfoStore(deg, -1, -1)
	allData, _, err := ei.Find(func(EventInfo) error { return nil })
	if err != nil {
		t.Fatalf("Unexpected error from find: %v", err)
	}
	checkEvIDEqual(t, allData, expectedFull[:10])
	subEv = totalEv[11:19]
	deg.setEv(12, subEv)
	_, _, err = ei.Find(func(EventInfo) error { return nil })
	if err == nil {
		t.Fatalf("Unexpected success from find")
	}
	ei.Cleanup()
}

// Test that two finds, with the second one having
// no new events, returns the right events.
func TestSequentialNoOp(t *testing.T) {
	totalEv := makeEvents()
	expectedFull := makeEvents()
	deg := newDummyEventGet()
	subEv := totalEv[:10]
	deg.setEv(1, subEv)
	ei := newTestableEventInfoStore(deg, -1, -1)
	allData, _, err := ei.Find(func(EventInfo) error { return nil })
	if err != nil {
		t.Fatalf("Unexpected error from find: %v", err)
	}
	checkEvIDEqual(t, allData, expectedFull[:10])
	subEv = []EventInfo{}
	deg.setEv(11, subEv)
	allData, _, err = ei.Find(func(EventInfo) error { return nil })
	if err != nil {
		t.Fatalf("Unexpected error from find: %v", err)
	}
	checkEvIDEqual(t, allData, expectedFull[:10])
	ei.Cleanup()
}

// Test that wait for N Works
func TestWaitForN(t *testing.T) {
	deg := newDummyEventGet()
	subEv := makeEvents()[:10]
	deg.setEv(1, subEv)
	ei := newTestableEventInfoStore(deg, time.Second/8, -1)
	var wg sync.WaitGroup
	wg.Add(1)
	var waitErr error
	var allMatch []EventInfo
	go func() {
		matchFunc := func(ev cloudevents.Event) error {
			if ev.ID() == "3" {
				return nil
			} else {
				return fmt.Errorf("mismatch %s %s", ev.ID(), "3")
			}
		}

		allMatch, waitErr = ei.WaitAtLeastNMatch(MatchEvent(matchFunc), 2)
		wg.Done()
	}()
	var tCalls int
	for tCalls == 0 {
		time.Sleep(time.Second / 100)
		tCalls = deg.trimCalls()
	}
	subEv = makeEvents()[:10]
	deg.setEv(11, subEv)

	wg.Wait()
	if waitErr != nil {
		t.Fatalf("Unexpected error from find: %v", waitErr)
	}
	if len(allMatch) != 2 {
		t.Fatalf("Unexpected match length: %d != %d", len(allMatch), 2)
	}
	if allMatch[0].Event.ID() != "3" || allMatch[1].Event.ID() != "3" {
		t.Fatalf("Unexpected IDs %s, %s, expected both %s", allMatch[0].Event.ID(), allMatch[1].Event.ID(), "3")
	}
	tCalls = deg.trimCalls()
	if tCalls < 2 {
		t.Fatalf("Expected at least %d trim calls, saw %d", 2, tCalls)
	}
	ei.Cleanup()
}
