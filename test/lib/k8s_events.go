/*
Copyright 2020 The Knative Authors

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

package lib

import (
	"context"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// EventHandler is the callback type for the EventListener
type EventHandler func(event *corev1.Event)

// EventListener is a type that broadcasts new k8s events
type EventListener struct {
	cancel context.CancelFunc

	lock     sync.Mutex
	handlers []EventHandler

	eventsSeen int
}

// NewEventListener creates a new event listener
func NewEventListener(client kubernetes.Interface, namespace string, logf func(string, ...interface{})) *EventListener {
	ctx, cancelCtx := context.WithCancel(context.Background())
	informerFactory := informers.NewSharedInformerFactoryWithOptions(
		client,
		0,
		informers.WithNamespace(namespace),
	)
	eventsInformer := informerFactory.Core().V1().Events().Informer()

	el := EventListener{
		cancel: cancelCtx,
	}

	eventsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			el.handle(obj.(*corev1.Event))
		},
	})

	go func() {
		eventsInformer.Run(ctx.Done())
		el.lock.Lock()
		defer el.lock.Unlock()
		logf("EventListener stopped, %v events seen", el.eventsSeen)
	}()

	return &el
}

func (el *EventListener) handle(event *corev1.Event) {
	el.lock.Lock()
	defer el.lock.Unlock()
	el.eventsSeen++
	for _, handler := range el.handlers {
		handler(event)
	}
}

func (el *EventListener) AddHandler(handler EventHandler) int {
	el.lock.Lock()
	defer el.lock.Unlock()
	el.handlers = append(el.handlers, handler)
	// Return the number of events that have already been seen. This helps debug scenarios where
	// the expected event was seen, but only before this handler was added.
	return el.eventsSeen
}

func (el *EventListener) Stop() {
	el.cancel()
}
