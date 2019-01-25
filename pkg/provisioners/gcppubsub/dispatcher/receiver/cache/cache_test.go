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
	"testing"
	"time"
)

type entry struct {
	// differentiator is used to differentiate two entries inside the tests.
	differentiator int
	stopCalled     bool
	stopCalledLock sync.Mutex
}

var _ Stoppable = &entry{}

func (e *entry) Stop() {
	e.stopCalledLock.Lock()
	defer e.stopCalledLock.Unlock()
	e.stopCalled = true
}

func (e *entry) getStopCalled() bool {
	e.stopCalledLock.Lock()
	defer e.stopCalledLock.Unlock()
	return e.stopCalled
}

type testClock struct {
	now     time.Time
	nowLock sync.Mutex
}

func (c *testClock) Now() time.Time {
	c.nowLock.Lock()
	defer c.nowLock.Unlock()
	return c.now
}

func (c *testClock) Add(d time.Duration) {
	c.nowLock.Lock()
	defer c.nowLock.Unlock()
	c.now = c.now.Add(d)
}

func TestTTL_Get(t *testing.T) {
	testCases := map[string]struct {
		present bool
	}{
		"present": {
			present: true,
		},
		"not present": {
			present: false,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			cache := NewTTL()
			k := "my-key"
			i := &entry{}

			if tc.present {
				cache.Insert(k, i)
			}

			cachedValue := cache.Get(k)

			if tc.present && cachedValue != i {
				t.Fatalf("Unexpected value from the cache. Expected %v. Actual %v", i, cachedValue)
			} else if !tc.present && cachedValue != nil {
				t.Fatalf("Unexpected value from the cache. Expected nil. Actual %v", cachedValue)
			}
		})
	}
}

func TestTTL_Get_ExtendsTTL(t *testing.T) {
	cache := NewTTL()
	k := "my-key"
	i := &entry{}
	cache.Insert(k, i)
	originalTTL := cache.items[key(k)].ttl
	time.Sleep(1 * time.Millisecond)
	cache.Get(k)
	newTTL := cache.items[key(k)].ttl
	if originalTTL == newTTL {
		t.Fatal("TTL was not updated after cache.Get()")
	}
}

func TestTTL_Insert(t *testing.T) {
	testCases := map[string]struct {
		present bool
	}{
		"present": {
			present: true,
		},
		"not present": {
			present: false,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			cache := NewTTL()
			k := "my-key"
			originalItem := &entry{
				differentiator: 1,
			}
			newItem := &entry{
				differentiator: 2,
			}

			// Just for sanity, to ensure later assertions are meaningful.
			if originalItem == newItem {
				t.Fatal("originalItem == newItem")
			}

			if tc.present {
				cache.Insert(k, originalItem)
			}

			cachedValue := cache.Insert(k, newItem)

			if tc.present && cachedValue != originalItem {
				t.Fatalf("Unexpected value from the cache. Expected %v. Actual %v", originalItem, cachedValue)
			} else if !tc.present && cachedValue != newItem {
				t.Fatalf("Unexpected value from the cache. Expected %v. Actual %v", newItem, cachedValue)
			}
		})
	}
}

func TestTTL_InsertNil(t *testing.T) {
	cache := NewTTL()

	const k = "my-key"
	if inserted := cache.Insert(k, nil); inserted != nil {
		t.Fatalf("Insert returned non-nil: %v", inserted)
	}

	// Use a backdoor to clean the cache, this demonstrates that we don't panic if nil is inserted.
	cache.deleteAllItems()

	// Insert nil, then a real item to show that it 'overwrites' the nil.
	i := &entry{
		differentiator: 1,
	}
	if inserted := cache.Insert(k, nil); inserted != nil {
		t.Fatalf("Insert returned non-nil: %v", inserted)
	}
	if inserted := cache.Insert(k, i); inserted != i {
		t.Fatalf("Insert did not return i: %v", inserted)
	}
	if cached := cache.Get(k); cached != i {
		t.Fatalf("Get did not return i: %v", cached)
	}
}

func TestTTL_Cull(t *testing.T) {
	cache := NewTTL()
	tc := &testClock{
		now: time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	cache.currentTime = tc.Now

	toBeCulledKey := "to-be-culled"
	toBeCulledItem := &entry{
		differentiator: 1,
	}

	cache.Insert(toBeCulledKey, toBeCulledItem)
	if i := cache.Get(toBeCulledKey); i == nil {
		t.Fatal("toBeCulled already culled, should still be present")
	}

	tc.Add(cache.ttl * 2)

	toBeRetainedKey := "to-be-retained"
	toBeRetainedItem := &entry{
		differentiator: 2,
	}
	cache.Insert(toBeRetainedKey, toBeRetainedItem)
	tc.Add(cache.ttl - 1)

	cache.cull()

	if i := cache.Get(toBeCulledKey); i != nil {
		t.Fatalf("Expected cache[toBeCulled] to be nil, actually %v", i)
	}
	if !toBeCulledItem.stopCalled {
		t.Fatal("toBeCulledItem stop was not called")
	}
	if i := cache.Get(toBeRetainedKey); i != toBeRetainedItem {
		t.Fatalf("Expected cache[toBeRetained] to be %v, actually %v", toBeRetainedItem, i)
	}
	if toBeRetainedItem.stopCalled {
		t.Fatal("toBeRetainedItem stop was called")
	}
}

func TestTTL_deleteUnderLock_NotPresent(t *testing.T) {
	// This isn't expected to happen with the current code structure, but to be more resilient to
	// future bugs, we check what happens when cache.deleteUnderLock() is called on a non-existent
	// key.
	cache := NewTTL()
	stopFn := cache.deleteUnderLock(key("not-in-cache"))
	stopFn()
}

func TestTTL_DeleteIfPresent(t *testing.T) {
	const k = "my-key"
	testCases := map[string]struct {
		deleteValueInCache bool
		otherValueInCache  bool
	}{
		"nothing in cache": {},
		"different item in cache": {
			otherValueInCache: true,
		},
		"deleted from cache": {
			deleteValueInCache: true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			cache := NewTTL()

			deleteValue := &entry{
				differentiator: 1,
			}
			otherValue := &entry{
				differentiator: deleteValue.differentiator + 1,
			}
			if tc.deleteValueInCache {
				cache.Insert(k, deleteValue)
			} else if tc.otherValueInCache {
				cache.Insert(k, otherValue)
			}

			cache.DeleteIfPresent(k, deleteValue)

			// First check the state of the cache.
			if tc.otherValueInCache {
				if actual := cache.Get(k); actual != otherValue {
					t.Errorf("otherValue should be in the cache, actual %v", actual)
				}
			} else if actual := cache.Get(k); actual != nil {
				t.Errorf("cache should have been empty, actually %v", actual)
			}

			// Now check the state Stop()'s.
			if tc.deleteValueInCache != deleteValue.stopCalled {
				t.Errorf("deleteValue's Stop() incorrect. Expected to be called %v. Actually called %v", tc.deleteValueInCache, deleteValue.stopCalled)
			}
			if otherValue.stopCalled {
				t.Errorf("otherValue's Stop() was called, it shouldn't have been")
			}
		})
	}
}

func TestTTL_Start(t *testing.T) {
	// Test to ensure start continuously culls in the background.
	cache := NewTTL()
	tc := &testClock{
		now: time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	cache.currentTime = tc.Now
	cache.ttl = 100 * time.Millisecond
	// Something small to make sure the test behaves well.
	cache.reapingPeriod = 1 * time.Millisecond

	stopCh := make(chan struct{})
	defer close(stopCh)
	go func() {
		_ = cache.Start(stopCh)
	}()

	const k = "my-key"
	i := &entry{
		differentiator: 1,
	}
	cache.Insert(k, i)

	for j := 0; j < 1; j++ {
		// cache.Get is called and extends the TTL.
		getAndAssertPresentAndNotStopped(t, cache, k, i)
		tc.Add(cache.ttl / 2)
		time.Sleep(cache.reapingPeriod * 10)
	}

	// Sleep the actual TTL, so now we have slept 3/2 * TTL in total, the entry should have been
	// culled.
	tc.Add(cache.ttl)
	time.Sleep(cache.reapingPeriod * 10)

	if c := cache.Get(k); c != nil {
		t.Fatal("Item still in the cache")
	}
	if !i.getStopCalled() {
		t.Fatal("Stop not called")
	}
}

func TestTTL_deleteAllItems(t *testing.T) {
	cache := NewTTL()

	const k1 = "my-key-1"
	i1 := &entry{
		differentiator: 1,
	}
	const k2 = "my-key-2"
	i2 := &entry{
		differentiator: 2,
	}

	cache.Insert(k1, i1)
	cache.Insert(k2, i2)

	if cache.Get(k1) != i1 || cache.Get(k2) != i2 {
		t.Fatalf("Cache did not have expected values. i1: %v, i2: %v", cache.Get(k1), cache.Get(k2))
	}

	stopCh := make(chan struct{})
	close(stopCh)
	if err := cache.Start(stopCh); err != nil {
		t.Fatalf("unexpected error from Start(): %v", err)
	}

	// First ensure nothing is in the cache anymore.
	if cache.Get(k1) != nil || cache.Get(k2) != nil {
		t.Fatalf("Cache had values, which it shouldn't. i1: %v, i2: %v", cache.Get(k1), cache.Get(k2))
	}

	// Now ensure everything was stopped.
	if !i1.stopCalled {
		t.Error("i1.Stop() not called")
	}
	if !i2.stopCalled {
		t.Error("i2.Stop() not called")
	}
}

func getAndAssertPresentAndNotStopped(t *testing.T, cache *TTL, k string, i *entry) {
	if c := cache.Get(k); i != c {
		t.Fatal("Item was not in the cache")
	} else if i.getStopCalled() {
		t.Fatal("Item's stop was called")
	}
}
