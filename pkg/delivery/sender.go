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

package delivery

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/elafros/eventing/pkg/delivery/action"
	"github.com/elafros/eventing/pkg/delivery/queue"
)

// Sender delivers queued events to their target Actions.
type Sender struct {
	eventQueue queue.Queue
	actions    map[string]action.Action
}

// NewSender creates a new Sender object to deliver Events which were
// enqueued by a Receiver.
func NewSender(
	eventQueue queue.Queue,
	actions map[string]action.Action) *Sender {
	return &Sender{
		eventQueue: eventQueue,
		actions:    actions,
	}
}

// RunOnce processes a single event from the queue.
// TODO: If the Queue was redesigned so that events are pulled with a lease, then we could restructure RunOnce to not
// need the stopCh (though does it really matter now anyway if the in-memory queue goes away?)
func (s *Sender) RunOnce(stopCh <-chan struct{}) bool {
	// TODO(inlined): Pull should only take a lease on the queued event. The receiver should need
	// to acknowledge the event and possibly transactionally enqueue subsequent events.
	event, ok := s.eventQueue.Pull(stopCh)
	if !ok {
		glog.V(4).Info("RunOnce shutting down")
		return false
	}

	glog.V(4).Info("Sending Event %s to %s Action %s", event.Context.EventID, event.Action.Processor, event.Action.Name)
	action, ok := s.actions[event.Action.Processor]
	if !ok {
		runtime.HandleError(fmt.Errorf("Event %s is routed to unknown Processor '%s'", event.Context.EventID, event.Action.Processor))
		return true
	}

	// TODO(inlined): retry strategies for errors and continuations for success.
	res, err := action.SendEvent(event.Action.Name, event.Data, event.Context)
	if err != nil {
		runtime.HandleError(fmt.Errorf("Action for event %s returned error: %s", event.Context.EventID, err))
	} else {
		glog.V(4).Info("Event %s handled with response %+v", event.Context.EventID, res)
	}
	return true
}

// Run runs Sender until stopCh is closed.
func (s *Sender) Run(threadiness int, stopCh <-chan struct{}) error {
	for i := 0; i < threadiness; i++ {
		go wait.Until(func() { s.RunOnce(stopCh) }, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}
