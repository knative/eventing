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

type anyFilter struct {
	filters []filterCount
	rwMutex sync.RWMutex
	c       chan int
	d       chan bool
}

// NewAnyFilter returns an event filter which passes if any of the contained filters passes.
func NewAnyFilter(filters ...eventfilter.Filter) eventfilter.Filter {
	filterCounts := make([]filterCount, len(filters))
	for i, filter := range filters {
		filterCounts[i] = filterCount{
			count:  *atomic.NewUint64(uint64(0)),
			filter: filter,
		}
	}
	f := &anyFilter{
		filters: filterCounts,
		c:       make(chan int),
		d:       make(chan bool),
	}
	go f.optimzeLoop()
	return f
}

func (filter *anyFilter) Filter(ctx context.Context, event cloudevents.Event) eventfilter.FilterResult {
	res := eventfilter.NoFilter
	logging.FromContext(ctx).Debugw("Performing an ANY match ", zap.Any("filters", filter), zap.Any("event", event))
	filter.rwMutex.RLock()
	defer filter.rwMutex.RUnlock()
	for i, f := range filter.filters {
		res = res.Or(f.filter.Filter(ctx, event))
		// Short circuit to optimize it
		if res == eventfilter.PassFilter {
			filter.c <- i
			return eventfilter.PassFilter
		}
	}
	return res
}

func (filter *anyFilter) Cleanup() {
	close(filter.c)
	<-filter.d
	for _, f := range filter.filters {
		f.filter.Cleanup()
	}
}

func (filter *anyFilter) optimzeLoop() {
	for {
		i, more := <-filter.c
		if !more {
			filter.d <- true
			return
		}
		val := filter.filters[i].count.Inc()
		if i != 0 && val > filter.filters[i-1].count.Load()*2 {
			go filter.optimize(i)
		}
	}
}

func (filter *anyFilter) optimize(swapIdx int) {
	filter.rwMutex.Lock()
	defer filter.rwMutex.Unlock()
	filter.filters[swapIdx-1], filter.filters[swapIdx] = filter.filters[swapIdx], filter.filters[swapIdx-1]
}

var _ eventfilter.Filter = &anyFilter{}
