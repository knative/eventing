/*
Copyright 2020 The Knative Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"knative.dev/eventing/test/lib/recordevents"
)

func helperGetEvInfo(es *eventStore, entry int) (*recordevents.EventInfo, error) {
	evInfoBytes, err := es.GetEventInfoBytes(entry)
	if err != nil {
		return nil, fmt.Errorf("error calling get on item %d: %v", entry, err)
	}
	if len(evInfoBytes) == 0 {
		return nil, fmt.Errorf("empty info bytes")
	}

	var evInfo recordevents.EventInfo
	err = json.Unmarshal(evInfoBytes, &evInfo)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling stored JSON: %v", err)
	}
	return &evInfo, nil
}

// Test that adding and getting a bunch of events stores them all
// and that retrieving the events retrieves the correct events.
func TestAddGetMany(t *testing.T) {
	es := newEventStore()

	count := 10009
	for i := 0; i < count; i++ {
		ce := cloudevents.NewEvent(cloudevents.VersionV1)
		ce.SetType("knative.dev.test.event.a")
		ce.SetSource("https://source.test.event.knative.dev/foo")
		ce.SetID(strconv.FormatInt(int64(i), 10))
		es.StoreEvent(&ce, nil, nil)
		minAvail, maxSeen := es.MinMax()
		if minAvail != 1 {
			t.Fatalf("Pass %d Bad min: %d, expected %d", i, minAvail, 1)
		}
		if maxSeen != i+1 {
			t.Fatalf("Pass %d Bad max: %d, expected %d", i, maxSeen, i+1)
		}

	}
	for i := 1; i <= count; i++ {
		evInfo, err := helperGetEvInfo(es, i)
		if err != nil {
			t.Fatalf("Count %d error: %v", count, err)
		}

		if evInfo.Event == nil {
			t.Fatalf("Unexpected empty event info event %d: %+v", i, evInfo)
		}
		if len(evInfo.Error) != 0 {
			t.Fatalf("Unexpected error for stored event %d: %s", i, evInfo.Error)
		}

		// Make sure it's the expected event
		seenID := evInfo.Event.ID()
		expectedID := strconv.FormatInt(int64(i-1), 10)
		if seenID != expectedID {
			t.Errorf("Incorrect id on retrieval: %s, expected %s", seenID, expectedID)
		}
	}
	_, err := es.GetEventInfoBytes(count + 1)
	if err == nil {
		t.Error("Unexpected non-error return for getinfo of", count+1)
	}

	_, err = es.GetEventInfoBytes(0)
	if err == nil {
		t.Error("Unexpected non-error return for getinfo of", 0)
	}

}

func TestEmpty(t *testing.T) {
	es := newEventStore()
	min, max := es.MinMax()
	if min != 1 {
		t.Errorf("Invalid min: %d, expected %d", min, 1)
	}
	if max != 0 {
		t.Errorf("Invalid max: %d, expected %d", max, 0)
	}

	for i := -2; i < 2; i++ {
		_, err := es.GetEventInfoBytes(0)
		if err == nil {
			t.Error("Unexpected non-error return for getinfo of", i)
		}
	}

}

func TestAddGetSingleValid(t *testing.T) {
	expectedType := "knative.dev.test.event.a"
	expectedSource := "https://source.test.event.knative.dev/foo"
	expectedID := "111"
	es := newEventStore()

	headers := make(map[string][]string)
	headers["foo"] = []string{"bar", "baz"}
	ce := cloudevents.NewEvent(cloudevents.VersionV1)
	ce.SetType(expectedType)
	ce.SetSource(expectedSource)
	ce.SetID(expectedID)
	es.StoreEvent(&ce, nil, headers)
	minAvail, maxSeen := es.MinMax()
	if minAvail != maxSeen {
		t.Fatalf("Expected match, saw %d, %d", minAvail, maxSeen)
	}

	evInfoBytes, err := es.GetEventInfoBytes(minAvail)
	if err != nil {
		t.Fatal("Error calling get:", err)
	}
	var evInfo recordevents.EventInfo
	err = json.Unmarshal(evInfoBytes, &evInfo)
	if err != nil {
		t.Fatal("Error unmarshalling stored JSON:", err)
	}

	if evInfo.Event == nil {
		t.Fatalf("Unexpected empty event info event: %+v", evInfo)
	}
	if len(evInfo.Error) != 0 {
		t.Fatal("Unexpected error for stored event:", evInfo.Error)
	}
	if len(evInfo.HTTPHeaders) != 1 {
		t.Fatalf("Unexpected header contents for stored event: %+v", evInfo.HTTPHeaders)
	}
	if len(evInfo.HTTPHeaders["foo"]) != 2 {
		t.Fatalf("Unexpected header contents for stored event: %+v", evInfo.HTTPHeaders)
	}
	if evInfo.HTTPHeaders["foo"][0] != "bar" || evInfo.HTTPHeaders["foo"][1] != "baz" {
		t.Fatalf("Unexpected header contents for stored event: %+v", evInfo.HTTPHeaders)
	}
	seenID := evInfo.Event.ID()
	if seenID != expectedID {
		t.Errorf("Incorrect id on retrieval: %s, expected %s", seenID, expectedID)
	}
	seenSource := evInfo.Event.Source()
	if seenSource != expectedSource {
		t.Errorf("Incorrect source on retrieval: %s, expected %s", seenSource, expectedSource)
	}
	seenType := evInfo.Event.Type()
	if seenType != expectedType {
		t.Errorf("Incorrect type on retrieval: %s, expected %s", seenType, expectedType)
	}
}

func TestAddGetSingleInvalid(t *testing.T) {
	es := newEventStore()

	headers := make(map[string][]string)
	headers["foo"] = []string{"bar", "baz"}
	ce := cloudevents.NewEvent(cloudevents.VersionV1)
	ce.SetType("knative.dev.test.event.a")
	// No source
	ce.SetID("111")
	es.StoreEvent(&ce, nil, headers)
	minAvail, maxSeen := es.MinMax()
	if minAvail != maxSeen {
		t.Fatalf("Expected match, saw %d, %d", minAvail, maxSeen)
	}

	evInfoBytes, err := es.GetEventInfoBytes(minAvail)
	if err != nil {
		t.Fatal("Error calling get:", err)
	}
	var evInfo recordevents.EventInfo
	err = json.Unmarshal(evInfoBytes, &evInfo)
	if err != nil {
		t.Fatal("Error unmarshalling stored JSON:", err)
	}
	if evInfo.Event != nil {
		t.Fatalf("Unexpected event info: %+v", evInfo)
	}
	if len(evInfo.Error) == 0 {
		t.Fatal("Unexpected empty error for stored event:", evInfo.Error)
	}
	if len(evInfo.HTTPHeaders) != 1 {
		t.Fatalf("Unexpected header contents for stored event: %+v", evInfo.HTTPHeaders)
	}
	if len(evInfo.HTTPHeaders["foo"]) != 2 {
		t.Fatalf("Unexpected header contents for stored event: %+v", evInfo.HTTPHeaders)
	}
	if evInfo.HTTPHeaders["foo"][0] != "bar" || evInfo.HTTPHeaders["foo"][1] != "baz" {
		t.Fatalf("Unexpected header contents for stored event: %+v", evInfo.HTTPHeaders)
	}
}

func TestAddGetSingleInvalidError(t *testing.T) {
	es := newEventStore()

	headers := make(map[string][]string)
	headers["foo"] = []string{"bar", "baz"}
	ce := cloudevents.NewEvent(cloudevents.VersionV1)
	ce.SetType("knative.dev.test.event.a")
	ce.SetID("111")
	ce.SetSource("nnn")
	es.StoreEvent(&ce, fmt.Errorf("Error passed in"), headers)
	minAvail, maxSeen := es.MinMax()
	if minAvail != maxSeen {
		t.Fatalf("Expected match, saw %d, %d", minAvail, maxSeen)
	}

	evInfoBytes, err := es.GetEventInfoBytes(minAvail)
	if err != nil {
		t.Fatal("Error calling get:", err)
	}
	var evInfo recordevents.EventInfo
	err = json.Unmarshal(evInfoBytes, &evInfo)
	if err != nil {
		t.Fatal("Error unmarshalling stored JSON:", err)
	}
	if evInfo.Event != nil {
		t.Fatalf("Unexpected event info: %+v", evInfo)
	}
	if len(evInfo.Error) == 0 {
		t.Fatal("Unexpected empty error for stored event:", evInfo.Error)
	}
	if len(evInfo.HTTPHeaders) != 1 {
		t.Fatalf("Unexpected header contents for stored event: %+v", evInfo.HTTPHeaders)
	}
	if len(evInfo.HTTPHeaders["foo"]) != 2 {
		t.Fatalf("Unexpected header contents for stored event: %+v", evInfo.HTTPHeaders)
	}
	if evInfo.HTTPHeaders["foo"][0] != "bar" || evInfo.HTTPHeaders["foo"][1] != "baz" {
		t.Fatalf("Unexpected header contents for stored event: %+v", evInfo.HTTPHeaders)
	}
}

func helperFillCount(es *eventStore, count int) {
	for i := 0; i < count; i++ {
		ce := cloudevents.NewEvent(cloudevents.VersionV1)
		ce.SetType("knative.dev.test.event.a")
		ce.SetSource("https://source.test.event.knative.dev/foo")
		ce.SetID(strconv.FormatInt(int64(i), 10))
		es.StoreEvent(&ce, nil, nil)
	}
}

// Test that adding and getting a bunch of events stores them all
// and that retrieving the events retrieves the correct events.
func TestTrim(t *testing.T) {
	count := evBlockSize + 10

	validTrimPoints := []int{0, 1, 2, evBlockSize - 1, evBlockSize, evBlockSize + 1, evBlockSize + 2, count - 1, count}
	invalidTrimPoints := []int{-2, -1, count + 1, count + evBlockSize}

	for _, testVal := range validTrimPoints {
		es := newEventStore()
		helperFillCount(es, count)
		err := es.TrimThrough(testVal)
		if err != nil {
			t.Fatalf("Unexpected error trimming to %d: %v", testVal, err)
		}
		minAvail, maxSeen := es.MinMax()
		if testVal == count {
			if minAvail != maxSeen+1 {
				t.Errorf("Incorrect minAvail (%d != %d+1) trimming to %d", minAvail, maxSeen, testVal)
			}
		} else if minAvail != testVal+1 {
			t.Errorf("Incorrect minAvail %d trimming to %d", minAvail, testVal)
		}
		if maxSeen != count {
			t.Errorf("Incorrect maxSeen %d trimming to %d", maxSeen, testVal)
		}
		if minAvail <= maxSeen {
			evInfo, err := helperGetEvInfo(es, minAvail)
			if err != nil {
				t.Fatalf("Couldn't get min avail %d, trim %d error: %v", minAvail, testVal, err)
			}
			seenID := evInfo.Event.ID()
			expectedID := strconv.FormatInt(int64(minAvail-1), 10)
			if seenID != expectedID {
				t.Fatalf("Expected ID %s, saw %s for ev %d, trim %d", expectedID, seenID, minAvail, testVal)
			}
		}
		ce := cloudevents.NewEvent(cloudevents.VersionV1)
		ce.SetType("knative.dev.test.event.a")
		ce.SetSource("https://source.test.event.knative.dev/foo")
		ce.SetID(strconv.FormatInt(int64(count), 10))
		es.StoreEvent(&ce, nil, nil)
		addedMinAvail, addedMaxSeen := es.MinMax()
		if addedMaxSeen != maxSeen+1 {
			t.Fatalf("Add after trim resulted in bad maxSeen: expected %d, saw %d for trim %d",
				maxSeen, addedMaxSeen, testVal)
		}
		if minAvail == -1 && addedMinAvail != addedMaxSeen {
			t.Errorf("Add after full trim resulted in bad minAvail: expected %d, saw %d for trim %d",
				addedMaxSeen, addedMinAvail, testVal)
		} else if minAvail != -1 && minAvail != addedMinAvail {
			t.Errorf("Add after partial trim resulted in bad minAvail: expected %d, saw %d for trim %d",
				minAvail, addedMinAvail, testVal)

		}

	}
	for _, testVal := range invalidTrimPoints {
		es := newEventStore()
		helperFillCount(es, count)
		err := es.TrimThrough(testVal)
		if err == nil {
			t.Fatal("Incorrect missing error trimming to", testVal)
		}
		minAvail, maxSeen := es.MinMax()
		if minAvail != 1 {
			t.Errorf("Incorrect minAvail %d trimming to %d", minAvail, testVal)
		}
		if maxSeen != count {
			t.Errorf("Incorrect maxSeen %d trimming to %d", maxSeen, testVal)
		}
	}
}
