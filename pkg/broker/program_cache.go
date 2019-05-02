/*
 * Copyright 2019 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package broker

import (
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/hashicorp/golang-lru"
)

const (
	// programCacheSize is the default size of the CEL program cache.
	programCacheSize = 100
)

var (
	programCache programCacheInterface
	cacheOnce    sync.Once
)

type cacheEntry struct {
	program   cel.Program
	parseData bool
	err       error
}

type programCacheInterface interface {
	Add(string, cacheEntry) bool
	Get(string) (cacheEntry, bool)
}

type fakeCache struct{}

func (c *fakeCache) Add(_ string, _ cacheEntry) bool {
	return false
}

func (c *fakeCache) Get(_ string) (cacheEntry, bool) {
	return cacheEntry{}, false
}

type lruCache struct {
	lru *lru.Cache
}

func newCache() programCacheInterface {
	// Currently this can only return an error if size is <=0
	lru, err := lru.New(programCacheSize)
	if err != nil {
		//TODO need a package-level logger here
		return &fakeCache{}
	}
	return &lruCache{
		lru: lru,
	}
}

func (c *lruCache) Add(k string, e cacheEntry) bool {
	return c.lru.Add(k, e)
}

func (c *lruCache) Get(k string) (cacheEntry, bool) {
	res, exists := c.lru.Get(k)
	if !exists {
		return cacheEntry{}, false
	}
	entry, ok := res.(cacheEntry)
	if !ok {
		//TODO need a package-level logger here
		return cacheEntry{}, false
	}
	return entry, ok
}

// TODO there's probably a more elegant way to do this than
// returning 3 vars
type cachePopulatingFunc func(string) (cel.Program, bool, error)

func getOrCacheProgram(expr string, pf cachePopulatingFunc) (cel.Program, bool, error) {
	cacheOnce.Do(func() {
		programCache = newCache()
	})

	entry, ok := programCache.Get(expr)
	if !ok {
		prg, parseData, err := pf(expr)
		// If there was an error, cache that error so we don't try to parse this
		// program every time.
		entry = cacheEntry{program: prg, parseData: parseData, err: err}
		programCache.Add(expr, entry)
	}

	if entry.err != nil {
		return nil, entry.parseData, entry.err
	}
	return entry.program, entry.parseData, nil
}
