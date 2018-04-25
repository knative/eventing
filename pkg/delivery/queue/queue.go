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

import (
	"github.com/elafros/eventing/pkg/apis/bind/v1alpha1"
	"github.com/elafros/eventing/pkg/event"
)

// QueuedEvent is the element saved to an EventQueue before delivery
// TODO(inlined): This should probably be made package protected;
// It may be reasonable to just return a tuple of these values from
// the Queue pull operations instead of defining the struct.
type QueuedEvent struct {
	Action  v1alpha1.BindAction `json:"action"`
	Data    interface{}         `json:"data"`
	Context *event.Context      `json:"context"`
}

// Queue implements basic features to allow asynchronous buffering of events.
type Queue interface {
	Push(event QueuedEvent) error
	Pull(stopCh <-chan struct{}) (event QueuedEvent, ok bool)
	Length() int
}
