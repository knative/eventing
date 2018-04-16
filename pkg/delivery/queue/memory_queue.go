/*
Copyright 2018 Google, Inc. All rights reserved.

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

package queue

import "errors"

// InMemoryQueue implements the queue interface with a memory buffer.
// Note: this isn't just a simple typedef for a queue because we will soon start
// experimenting with other features, such as fetching an event separate from acking,
// transactional ack + enqueue, cursors, etc.
type InMemoryQueue struct {
	ch chan QueuedEvent
}

// NewInMemoryQueue creates a new InMemoryQueue
func NewInMemoryQueue(length int) *InMemoryQueue {
	ch := make(chan QueuedEvent, length)
	return &InMemoryQueue{
		ch: ch,
	}
}

// Push implements Queue.Push
func (q *InMemoryQueue) Push(event QueuedEvent) error {
	select {
	case q.ch <- event:
		return nil
	default:
		return errors.New("Event queue is out of memory")
	}
}

// Pull implements Queue.Pull
func (q *InMemoryQueue) Pull(stopCh <-chan struct{}) (QueuedEvent, bool) {
	select {
	case event, ok := <-q.ch:
		return event, ok
	case <-stopCh:
		return QueuedEvent{}, false
	}
}

// Length returns Queue.Length
func (q InMemoryQueue) Length() int {
	return len(q.ch)
}
