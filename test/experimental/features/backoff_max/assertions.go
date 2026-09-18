/*
Copyright 2026 The Knative Authors

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

package backoff_max

import (
	"fmt"
	"sort"
	"sync"
	"time"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
)

func assertBackoffMax(f *feature.Feature, receiverName, eventID string) {
	f.Assert("receiver rejects the first four deliveries", assert.OnStore(receiverName).
		MatchRejectedEvent(cetest.HasId(eventID)).Exact(4))
	f.Assert("receiver accepts the fifth delivery", assert.OnStore(receiverName).
		MatchReceivedEvent(cetest.HasId(eventID)).Exact(1))
	f.Assert("retry delay stops growing at two seconds", assert.OnStore(receiverName).
		Match(deliveriesFollowBackoff(eventID, []time.Duration{time.Second, 2 * time.Second, 2 * time.Second, 2 * time.Second})).Exact(5))
}

func deliveriesFollowBackoff(id string, expected []time.Duration) eventshub.EventInfoMatcher {
	type deliveryKey struct {
		kind     eventshub.EventKind
		sequence uint64
	}

	var mu sync.Mutex
	seen := make(map[deliveryKey]eventshub.EventInfo, len(expected)+1)

	return func(info eventshub.EventInfo) error {
		if info.Event == nil || info.Event.ID() != id {
			return fmt.Errorf("received a different event")
		}

		mu.Lock()
		defer mu.Unlock()
		seen[deliveryKey{kind: info.Kind, sequence: info.Sequence}] = info
		if len(seen) < len(expected)+1 {
			return nil
		}

		deliveries := make([]eventshub.EventInfo, 0, len(seen))
		for _, delivery := range seen {
			deliveries = append(deliveries, delivery)
		}
		sort.Slice(deliveries, func(i, j int) bool {
			return deliveries[i].Time.Before(deliveries[j].Time)
		})

		for i, wait := range expected {
			actual := deliveries[i+1].Time.Sub(deliveries[i].Time)
			if actual < wait-500*time.Millisecond || actual > wait+3*time.Second {
				return fmt.Errorf("delivery %d waited %s, expected %s", i+2, actual, wait)
			}
		}
		return nil
	}
}
