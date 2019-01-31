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

package cache

import (
	"sync"
	"time"
)

const (
	// stopSliceSize is the initial size of the slice for storing Stop() functions of culled
	// entries. It was chosen somewhat at random. It should be large enough that we don't need to
	// extend it while appending under lock. It should be small enough not to take up a huge amount
	// of memory.
	stopSliceSize = 1024
)

// key is the key of each cache entry.
type key string

// TTL is a time-to-live cache. Entries added to it are automatically culled after a period of
// inactivity. When evicted from the cache, the entry's Stop() method will be called.
//
// Example usage:
//
//    cache := cache.NewTTL()
//    go cache.Start(stopCh)
//    cachedValue := cache.Insert(key, value)
//    cachedValue.Start()
//    otherValue := cache.Get(otherKey)
//    if otherValue != nil {
//        ...use otherValue...
//    }
//    ...cachedValue times out and cachedValue.Stop() is called in a background goroutine...
type TTL struct {
	itemsLock     sync.Mutex
	items         map[key]*item
	ttl           time.Duration
	reapingPeriod time.Duration
	currentTime   func() time.Time
}

// item is stored in the cache.TTL's items map. It represents the value stored in the cache, as well
// as the time it lives until.
type item struct {
	value Stoppable
	ttl   time.Time
}

// Stoppable is the interface that wraps a basic Stop method.
type Stoppable interface {
	// Stop is the method called when an item is evicted from the cache.
	Stop()
}

// NewTTL creates a new cache.TTL. Note that for the culling to occur, Start(stopCh) must be called.
//
//     cache := cache.NewTTL()
//     go cache.Start(stopCh)
func NewTTL() *TTL {
	return &TTL{
		itemsLock:     sync.Mutex{},
		items:         make(map[key]*item),
		ttl:           5 * time.Minute,
		reapingPeriod: 5 * time.Minute,
		currentTime:   time.Now,
	}
}

// Insert inserts an item into the cache. If something is already stored with that key, then the
// existing stored value is returned. This is especially important if the value needs to be started,
// in which case the returned value must be started, not the input.
//
//     cachedValue := cache.Insert(key, value)
//     // Note that cachedValue may either be value or something that was already in the cache.
//     cachedValue.Start()
func (c *TTL) Insert(keyString string, value Stoppable) Stoppable {
	// Don't let nil values in, it will cause panics when they get Stop()ed. As well as blocking a
	// non-nil value from getting in.
	if value == nil {
		return nil
	}
	newItem := &item{
		value: value,
		ttl:   c.currentTime().Add(c.ttl),
	}
	k := key(keyString)
	c.itemsLock.Lock()
	defer c.itemsLock.Unlock()
	current, present := c.items[k]
	if present {
		current.extend(c.currentTime, c.ttl)
		return current.value
	} else {
		c.items[k] = newItem
		return value
	}
}

// Get gets an item in the cache and extends its time to live. If the key is not present in the
// cache, nil is returned.
func (c *TTL) Get(keyString string) Stoppable {
	k := key(keyString)
	c.itemsLock.Lock()
	defer c.itemsLock.Unlock()
	i := c.items[k]
	if i != nil {
		i.extend(c.currentTime, c.ttl)
		return i.value
	}
	return nil
}

// extend extends the time this item's time to live to now plus ttl.
func (i *item) extend(currentTime func() time.Time, ttl time.Duration) {
	i.ttl = currentTime().Add(ttl)
}

// DeleteIfPresent deletes an entry from the cache if its value in the cache is equal to value.
func (c *TTL) DeleteIfPresent(keyString string, value Stoppable) {
	stoppable := func() Stoppable {
		c.itemsLock.Lock()
		defer c.itemsLock.Unlock()
		k := key(keyString)
		actualValue, present := c.items[k]
		if present {
			if actualValue.value == value {
				delete(c.items, k)
				return value
			}
		}
		return nil
	}()
	if stoppable != nil {
		stoppable.Stop()
	}
}

// Start continuously culls the old items from the cache. It should be run in a goroutine.
func (c *TTL) Start(stopCh <-chan struct{}) error {
	for {
		select {
		case <-stopCh:
			c.deleteAllItems()
			return nil
		case <-time.After(c.reapingPeriod):
			c.cull()
		}
	}
}

// cull all items from the cache that are past their time to live.
func (c *TTL) cull() {
	// Minimize time when the lock is held by collecting all the functions that will be stopped, but
	// not running them, under lock.
	stops := func() []func() {
		stops := make([]func(), 0, stopSliceSize)
		now := c.currentTime()
		c.itemsLock.Lock()
		defer c.itemsLock.Unlock()
		for k, i := range c.items {
			if i.ttl.Before(now) {
				stops = append(stops, c.deleteUnderLock(k))
			}
		}
		return stops
	}()
	for _, stop := range stops {
		stop()
	}
}

// deleteUnderLock deletes the item from the cache, if it is present. It returns the Stop() function
// that should be called to actually stop the entry.
//
// This function can only be called if the c.itemsLock lock is held.
func (c *TTL) deleteUnderLock(k key) func() {
	i, present := c.items[k]
	if present {
		delete(c.items, k)
		return i.value.Stop
	}
	// I don't think this can happen, as this can only be called under lock and should only be
	// called if the key is present, but just in case return a nop function.
	return func() {}
}

// deleteAllItems is expected to be called only when the cache is stopped. It removes all items from
// the cache and Stop()s them.
func (c *TTL) deleteAllItems() {
	oldItems := func() map[key]*item {
		c.itemsLock.Lock()
		defer c.itemsLock.Unlock()
		oldItems := c.items
		// Any subsequent cache gets should not return any of these items, so just make a new map.
		c.items = make(map[key]*item)
		return oldItems
	}()
	for _, stop := range oldItems {
		stop.value.Stop()
	}
}
