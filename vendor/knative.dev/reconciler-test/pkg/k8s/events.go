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

package k8s

import (
	"context"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"

	"knative.dev/reconciler-test/pkg/environment"
)

type eventListenerEnvKey struct{}

func WithEventListener(ctx context.Context, env environment.Environment) (context.Context, error) {
	return context.WithValue(
		ctx,
		eventListenerEnvKey{},
		newEventListener(kubeclient.Get(ctx), env.Namespace(), logging.FromContext(ctx).Infof),
	), nil
}

var _ environment.EnvOpts = WithEventListener

func EventListenerFromContext(ctx context.Context) *EventListener {
	if e, ok := ctx.Value(eventListenerEnvKey{}).(*EventListener); ok {
		return e
	}
	panic("no event listener found in the context, make sure you properly configured the env opts using WithEventListener")
}

// EventHandler is the callback type for the EventListener
type EventHandler interface {
	Handle(event *corev1.Event)
}

// EventListener is a type that broadcasts new k8s events to subscribed EventHandler
// The scope of EventListener should be global in the Environment lifecycle
type EventListener struct {
	cancel context.CancelFunc

	lock     sync.Mutex
	handlers map[string]EventHandler

	eventsSeen int
}

func newEventListener(client kubernetes.Interface, namespace string, logf func(string, ...interface{})) *EventListener {
	ctx, cancelCtx := context.WithCancel(context.Background())
	informerFactory := informers.NewSharedInformerFactoryWithOptions(
		client,
		0,
		informers.WithNamespace(namespace),
	)
	eventsInformer := informerFactory.Core().V1().Events().Informer()

	el := EventListener{
		cancel:   cancelCtx,
		handlers: make(map[string]EventHandler),
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
		handler.Handle(event)
	}
}

func (el *EventListener) GetHandler(name string) EventHandler {
	el.lock.Lock()
	defer el.lock.Unlock()
	return el.handlers[name]
}

func (el *EventListener) AddHandler(name string, handler EventHandler) int {
	el.lock.Lock()
	defer el.lock.Unlock()
	el.handlers[name] = handler
	// Return the number of events that have already been seen. This helps debug scenarios where
	// the expected event was seen, but only before this handler was added.
	return el.eventsSeen
}

func (el *EventListener) Stop() {
	el.cancel()
}
