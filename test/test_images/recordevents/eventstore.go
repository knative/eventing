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

package main

import (
	"encoding/json"
	"fmt"
	"sync"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"knative.dev/eventing/test/lib/recordevents"
)

// Number of EventInfo per block
const evBlockSize = 100

// Block of stored EventInfo
type eventBlock struct {
	// seqno of [0] evInfoBytes entry
	firstIndex int
	// offset inside block for newly appended event
	firstOffsetFree int
	// offset inside block of first non-trimmed event
	firstValid int
	// serialized EventInfo structures for each seqno.  We enforce
	// that there is always at least one block.
	evInfoBytes [evBlockSize][]byte
}

// All events currently seen
type eventStore struct {
	// Blocks of events in increasing sequency number order
	evBlocks     []*eventBlock
	evBlocksLock sync.Mutex
}

// Create a new event store.
func newEventStore() *eventStore {
	es := &eventStore{}
	es.evBlocks = []*eventBlock{{}}

	// One block with no entries starting at sequence number 1
	es.evBlocks[0].firstIndex = 1
	es.evBlocks[0].firstOffsetFree = 0
	es.evBlocks[0].firstValid = 0
	return es
}

// See if there's enough room to append to the current last block.  If not,
// append an extra block.
func (es *eventStore) checkAppendBlock() {
	if es.evBlocks[len(es.evBlocks)-1].firstOffsetFree == evBlockSize {
		newEVBlock := &eventBlock{
			firstIndex: es.evBlocks[len(es.evBlocks)-1].firstIndex + evBlockSize,
		}
		es.evBlocks = append(es.evBlocks, newEVBlock)
	}
}

// Store the specified event.
func (es *eventStore) StoreEvent(event *cloudevents.Event, evErr error, httpHeaders map[string][]string) {
	var evInfo recordevents.EventInfo
	var err error
	var evInfoBytes []byte
	if evErr != nil {
		evInfo.HTTPHeaders = httpHeaders
		evInfo.Error = evErr.Error()
		if evInfo.Error == "" {
			evInfo.Error = "Unknown Incoming Error"
		}
		evInfoBytes, err = json.Marshal(&evInfo)
		if err != nil {
			panic(fmt.Errorf("unexpected marshal error (%v) (%+v)", err, evInfo))
		}
	} else {
		evInfo.Event = event
		evInfo.HTTPHeaders = httpHeaders
		evInfoBytes, err = json.Marshal(&evInfo)

		if err != nil {
			evInfo.Event = nil
			evInfo.Error = err.Error()
			if evInfo.Error == "" {
				evInfo.Error = "Unknown Error"
			}
			evInfoBytes, err = json.Marshal(&evInfo)
			if err != nil {
				panic(fmt.Errorf("unexpected marshal error (%v) (%+v)", err, evInfo))
			}
		}
	}

	es.evBlocksLock.Lock()
	// Add a new block if we're out of space
	es.checkAppendBlock()

	evBlock := es.evBlocks[len(es.evBlocks)-1]
	if evBlock.firstOffsetFree < evBlockSize {
		evBlock.evInfoBytes[evBlock.firstOffsetFree] = evInfoBytes
		evBlock.firstOffsetFree++
	}

	es.evBlocksLock.Unlock()
}

// Logically trim all events up to and include the provided
// sequence number.  Returns error for patently incorrect
// values (negative sequence number or sequence number larger
// than the largest event seen).  Trimming already trimmed
// regions is legal.
func (es *eventStore) TrimThrough(through int) error {
	es.evBlocksLock.Lock()
	defer es.evBlocksLock.Unlock()
	minAvail, maxSeen := es.minMaxUnlocked()
	if through > maxSeen {
		return fmt.Errorf("invalid trim through %d, maxSeen %d", through, maxSeen)
	} else if through < 0 {
		return fmt.Errorf("invalid trim less than zero %d", through)
	} else if through < minAvail {
		return nil
	}
	// Completely remove blocks if they are full and all events in them are less than
	// the specified value.
	for len(es.evBlocks) > 1 && (es.evBlocks[0].firstIndex+evBlockSize-1) <= through {
		es.evBlocks = es.evBlocks[1:]
	}
	// Logically trim the block split by through.
	es.evBlocks[0].firstValid = through - es.evBlocks[0].firstIndex + 1
	return nil
}

// return min/max untrimmed value when already holding the lock
func (es *eventStore) minMaxUnlocked() (minAvail int, maxSeen int) {
	minBlock := es.evBlocks[0]
	minAvail = minBlock.firstIndex + (minBlock.firstValid)

	maxBlock := es.evBlocks[len(es.evBlocks)-1]
	maxSeen = maxBlock.firstIndex + maxBlock.firstOffsetFree - 1
	return minAvail, maxSeen
}

// Returns min available value and max seen value for the store.
// min is the minimum value that can be retrieved via GetEntry, or
// if no values can be retrieved, min == max+1.  Max starts at 0
// when no events have been seen.
func (es *eventStore) MinMax() (minAvail int, maxSeen int) {
	es.evBlocksLock.Lock()
	minAvail, maxSeen = es.minMaxUnlocked()

	es.evBlocksLock.Unlock()
	return minAvail, maxSeen
}

// Get the already serialized EventInfo structure for the provided sequence
// number.
func (es *eventStore) GetEventInfoBytes(seq int) ([]byte, error) {
	var evInfoBytes []byte
	found := false

	es.evBlocksLock.Lock()
	for _, block := range es.evBlocks {
		if seq < block.firstIndex+block.firstValid {
			break
		}
		if seq < block.firstIndex+block.firstOffsetFree {
			found = true
			evInfoBytes = block.evInfoBytes[seq-block.firstIndex]
			break
		}
	}
	es.evBlocksLock.Unlock()
	if !found {
		return evInfoBytes, fmt.Errorf("Invalid sequence number %d", seq)
	}
	return evInfoBytes, nil
}
