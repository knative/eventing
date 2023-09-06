/*
Copyright 2023 The Knative Authors

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
	"fmt"
	"sync"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/eventfilter"
)

type FiltersMap struct {
	filtersMap map[string]eventfilter.Filter
	rwMutex    sync.RWMutex
}

func NewFiltersMap() *FiltersMap {
	return &FiltersMap{
		filtersMap: make(map[string]eventfilter.Filter),
	}
}
func (fm *FiltersMap) Set(trigger *eventingv1.Trigger, filter eventfilter.Filter) {
	key := keyFromTrigger(trigger)
	fm.rwMutex.Lock()
	defer fm.rwMutex.Unlock()
	if filter, found := fm.filtersMap[key]; found {
		filter.Cleanup()
	}
	fm.filtersMap[key] = filter
}

func (fm *FiltersMap) Get(trigger *eventingv1.Trigger) (eventfilter.Filter, bool) {
	key := keyFromTrigger(trigger)
	fm.rwMutex.RLock()
	defer fm.rwMutex.RUnlock()
	res, found := fm.filtersMap[key]
	return res, found
}

func (fm *FiltersMap) Delete(trigger *eventingv1.Trigger) {
	key := keyFromTrigger(trigger)
	fm.rwMutex.Lock()
	defer fm.rwMutex.Unlock()
	if filter, found := fm.filtersMap[key]; found {
		filter.Cleanup()
	}
	delete(fm.filtersMap, key)
}

func keyFromTrigger(trigger *eventingv1.Trigger) string {
	return fmt.Sprintf("%s.%s", trigger.Namespace, trigger.Name)
}
