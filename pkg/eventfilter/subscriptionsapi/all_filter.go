/*
Copyright 2022 The Knative Authors

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

package subscriptionsapi

import (
	"context"
	"sync"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/eventfilter"
)

type filterCount struct {
	filter eventfilter.Filter
	count  atomic.Uint64
}

type allFilter struct {
	filters   []filterCount
	rwMutex   sync.RWMutex
	indexChan chan int
	doneChan  chan bool
}

// NewAllFilter returns an event filter which passes if all the contained filters pass
func NewAllFilter(filters ...eventfilter.Filter) eventfilter.Filter {
	// just one filter, no need to create a new AllFilter
	if len(filters) == 1 {
		return filters[0]
	}

	filterCounts := make([]filterCount, len(filters))
	for i, filter := range filters {
		filterCounts[i] = filterCount{
			count:  *atomic.NewUint64(uint64(0)),
			filter: filter,
		}
	}

	f := &allFilter{
		filters:   filterCounts,
		indexChan: make(chan int, 1),
		doneChan:  make(chan bool),
	}
	go f.optimizeLoop()
	return f
}

func (filter *allFilter) Filter(ctx context.Context, event cloudevents.Event) eventfilter.FilterResult {
	res := eventfilter.NoFilter
	logging.FromContext(ctx).Debugw("Performing an ALL match ", zap.Any("filters", filter), zap.Any("event", event))
	filter.rwMutex.RLock()
	defer filter.rwMutex.RUnlock()
	for i, f := range filter.filters {
		res = f.filter.Filter(ctx, event)
		// Short circuit to optimize it
		if res == eventfilter.FailFilter {
			select {
			// don't block if the channel is full
			case filter.indexChan <- i:
			default:
			}
			return eventfilter.FailFilter
		}
	}
	return res
}

func (filter *allFilter) optimizeLoop() {
	for {
		i, more := <-filter.indexChan
		if !more {
			filter.doneChan <- true
			return
		}
		val := filter.filters[i].count.Inc()
		if i != 0 && val > filter.filters[i-1].count.Load()*2 {
			go filter.swapWithEarlierFilter(i)
		}
	}
}

func (filter *allFilter) swapWithEarlierFilter(swapIdx int) {
	filter.rwMutex.Lock()
	defer filter.rwMutex.Unlock()
	filter.filters[swapIdx-1], filter.filters[swapIdx] = filter.filters[swapIdx], filter.filters[swapIdx-1]
}

func (filter *allFilter) Cleanup() {
	close(filter.indexChan)
	<-filter.doneChan
	for _, f := range filter.filters {
		f.filter.Cleanup()
	}
}

var _ eventfilter.Filter = &allFilter{}
