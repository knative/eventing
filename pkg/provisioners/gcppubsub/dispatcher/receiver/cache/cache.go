package cache

import (
	"sync"
	"time"
)

type Key string

type TTL struct {
	itemsLock     sync.Mutex
	items         map[Key]*Item
	ttl           time.Duration
	reapingPeriod time.Duration
}

type Item struct {
	value Stoppable
	ttl   time.Time
}

type Stoppable interface {
	Stop()
}

func NewTTL() *TTL {
	return &TTL{
		itemsLock:     sync.Mutex{},
		items:         make(map[Key]*Item),
		ttl:           5 * time.Minute,
		reapingPeriod: 5 * time.Minute,
	}
}

func (c *TTL) Insert(key string, value Stoppable) Stoppable {
	item := &Item{
		value: value,
		ttl:   time.Now().Add(c.ttl),
	}
	k := Key(key)
	c.itemsLock.Lock()
	defer c.itemsLock.Unlock()
	current, present := c.items[k]
	if present {
		current.extend(c.ttl)
		return current.value
	} else {
		c.items[k] = item
		return value
	}
}

func (c *TTL) deleteUnderLock(key Key) func() {
	item, present := c.items[key]
	if present {
		delete(c.items, key)
		return item.value.Stop
	}
	return func() {}
}

func (c *TTL) Get(key string) Stoppable {
	k := Key(key)
	c.itemsLock.Lock()
	defer c.itemsLock.Unlock()
	item := c.items[k]
	if item != nil {
		item.extend(c.ttl)
		return item.value
	}
	return nil
}

func (c *TTL) Start(stopCh <-chan struct{}) error {
	for {
		select {
		case <-stopCh:
			return nil
		case <-time.After(c.reapingPeriod):
			c.cull()
		}
	}
}

func (c *TTL) cull() {
	stops := func() []func() {
		stops := make([]func(), 0, 10)
		now := time.Now()
		c.itemsLock.Lock()
		defer c.itemsLock.Unlock()
		for key, item := range c.items {
			if item.ttl.Before(now) {
				stops = append(stops, c.deleteUnderLock(key))
			}
		}
		return stops
	}()
	for _, stop := range stops {
		stop()
	}
}

func (i *Item) extend(ttl time.Duration) {
	i.ttl = time.Now().Add(ttl)
}
