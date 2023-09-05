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
	"sync"

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
func (fm *FiltersMap) Set(key string, filter eventfilter.Filter) {
	fm.rwMutex.Lock()
	defer fm.rwMutex.Unlock()
	if filter, found := fm.filtersMap[key]; found {
		filter.Cleanup()
	}
	fm.filtersMap[key] = filter
}

func (fm *FiltersMap) Get(key string) (eventfilter.Filter, bool) {
	fm.rwMutex.RLock()
	defer fm.rwMutex.RUnlock()
	res, found := fm.filtersMap[key]
	return res, found
}

func (fm *FiltersMap) Delete(key string) {
	fm.rwMutex.Lock()
	defer fm.rwMutex.Unlock()
	if filter, found := fm.filtersMap[key]; found {
		filter.Cleanup()
	}
	delete(fm.filtersMap, key)
}
