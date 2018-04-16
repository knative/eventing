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
	"net/http"
	"regexp"

	"github.com/elafros/eventing/pkg/apis/bind/v1alpha1"

	listers "github.com/elafros/eventing/pkg/client/listers/bind/v1alpha1"
	"github.com/elafros/eventing/pkg/delivery/queue"
	"github.com/elafros/eventing/pkg/event"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/errors"
)

// If an event source knows the exact flow it is targeting it can bypass the work involved with
// processing event triggers.
var directSendEventRegExp = regexp.MustCompile(`^.*/namespaces/([^/]*)/flows/([^/]*):sendEvent$`)

// TODO(vaikas): Remove this once Bind's Action has been migrated
// to be generic.
const alwaysUseProcessor = "eventing.elafros.dev/EventLogger"

func actionFromBind(bind *v1alpha1.Bind) queue.ActionType {
	return queue.ActionType{
		Name:      bind.Spec.Action.RouteName,
		Processor: alwaysUseProcessor,
	}
}

// Receiver manages the HTTP endpoints for receiving events as well as
// stats about the queue for processing events.
type Receiver struct {
	eventQueue  queue.Queue
	bindsLister listers.BindLister
}

// NewReceiver creates a new Reciever object to enqueue events.
func NewReceiver(bindsLister listers.BindLister, eventQueue queue.Queue) *Receiver {
	return &Receiver{bindsLister: bindsLister, eventQueue: eventQueue}
}

// SendEvent enqueues an event data and Context for delivery to a particular action.
func (r *Receiver) SendEvent(action queue.ActionType, data interface{}, context *event.Context) error {
	return r.eventQueue.Push(queue.QueuedEvent{
		Action:  action,
		Data:    data,
		Context: context,
	})
}

// ServeHTTP implements the external REST API that other services will use to send events.
func (r *Receiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		glog.V(3).Info("Cannot handle method", req.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	matches := directSendEventRegExp.FindStringSubmatch(req.URL.Path)
	if len(matches) != 3 {
		glog.V(3).Info("Cannot parse route", req.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	bind, err := r.bindsLister.Binds(string(matches[1])).Get(string(matches[2]))
	if err != nil {
		if errors.IsNotFound(err) {
			fmt.Printf("Could not find Bind %s in namespace %s\n", matches[2], matches[1])
			glog.V(3).Infof("Event sent to non-existant Bind %s in namespace %s", matches[2], matches[1])
			w.WriteHeader(http.StatusNotFound)
			return
		}
		glog.Errorf("Unknown error fetching bind %s in namespace %s: %s", matches[2], matches[1], err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var data interface{}
	context, err := event.FromRequest(&data, req)
	if err != nil {
		glog.Error("Failed to unmarshal event ", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	if err := r.SendEvent(actionFromBind(bind), data, context); err != nil {
		glog.Error("Failed to enqueue event", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
