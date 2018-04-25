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
	"net/http"
	"regexp"

	"github.com/elafros/eventing/pkg/apis/bind/v1alpha1"

	listers "github.com/elafros/eventing/pkg/client/listers/bind/v1alpha1"
	"github.com/elafros/eventing/pkg/delivery/queue"
	"github.com/elafros/eventing/pkg/event"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/errors"
)

// Currently the delivery service requires that Events are sent directly to an endpoint named
// after a Bind which the Event matches. Future versions may offer a "firehose" endpoint that
// will do late-time filtering of Events.
var directSendEventRegExp = regexp.MustCompile(`^.*/namespaces/([^/]*)/flows/([^/]*):sendEvent$`)

// Receiver manages the HTTP endpoints for receiving events as well as
// stats about the queue for processing events.
type Receiver struct {
	eventQueue  queue.Queue
	bindsLister listers.BindLister
}

// NewReceiver creates a new Receiver object to enqueue events.
func NewReceiver(bindsLister listers.BindLister, eventQueue queue.Queue) *Receiver {
	return &Receiver{bindsLister: bindsLister, eventQueue: eventQueue}
}

// SendEvent enqueues an event data and Context for delivery to a particular action.
func (r *Receiver) SendEvent(action v1alpha1.BindAction, data interface{}, context *event.Context) error {
	return r.eventQueue.Push(queue.QueuedEvent{
		Action:  action,
		Data:    data,
		Context: context,
	})
}

// ServeHTTP implements the external REST API that other services will use to send events.
func (r *Receiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	glog.Infof("Serving %s", req.URL.Path)
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

	namespace, flowName := string(matches[1]), string(matches[2])
	bind, err := r.bindsLister.Binds(namespace).Get(flowName)
	if err != nil {
		if errors.IsNotFound(err) {
			glog.V(3).Infof("Event sent to non-existant Bind %s in namespace %s", flowName, namespace)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		glog.Errorf("Unknown error fetching bind %s in namespace %s: %s", flowName, namespace, err)
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

	if err := r.SendEvent(bind.Spec.Action, data, context); err != nil {
		glog.Error("Failed to enqueue event", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	glog.Infof("Enqueued event %s for delivery to %s action %s",
		context.EventID, bind.Spec.Action.Processor, bind.Spec.Action.Name)
	w.WriteHeader(http.StatusOK)
}
