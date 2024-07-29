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

type anyFilter struct {
	filters   []filterCount
	rwMutex   sync.RWMutex
	indexChan chan int
	doneChan  chan bool
}

// NewAnyFilter returns an event filter which passes if any of the contained filters passes.
func NewAnyFilter(filters ...eventfilter.Filter) eventfilter.Filter {
	// just one filter, no need to create a new AnyFilter
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
	f := &anyFilter{
		filters:   filterCounts,
		indexChan: make(chan int, 1),
		doneChan:  make(chan bool),
	}
	go f.optimzeLoop() // this goroutine is cleaned up when the Cleanup() method is called on the any filter
	return f
}

func (filter *anyFilter) Filter(ctx context.Context, event cloudevents.Event) eventfilter.FilterResult {
	res := eventfilter.NoFilter
	logging.FromContext(ctx).Debugw("Performing an ANY match ", zap.Any("filters", filter), zap.Any("event", event))
	filter.rwMutex.RLock()
	defer filter.rwMutex.RUnlock()
	for i, f := range filter.filters {
		res = f.filter.Filter(ctx, event)
		// Short circuit to optimize it
		if res == eventfilter.PassFilter {
			select {
			// don't block if the channel is full
			case filter.indexChan <- i:
			default:
			}
			return eventfilter.PassFilter
		}
	}
	return res
}

func (filter *anyFilter) Cleanup() {
	close(filter.indexChan)
	<-filter.doneChan
	for _, f := range filter.filters {
		f.filter.Cleanup()
	}
}

func (filter *anyFilter) optimzeLoop() {
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

func (filter *anyFilter) swapWithEarlierFilter(swapIdx int) {
	filter.rwMutex.Lock()
	defer filter.rwMutex.Unlock()
	filter.filters[swapIdx-1], filter.filters[swapIdx] = filter.filters[swapIdx], filter.filters[swapIdx-1]
}

var _ eventfilter.Filter = &anyFilter{}
