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
		logf("EventListener stopped")
	}()

	return &el
}

func (el *EventListener) handle(event *corev1.Event) {
	el.lock.Lock()
	defer el.lock.Unlock()
	for _, handler := range el.handlers {
		handler(event)
	}
}

func (el *EventListener) AddHandler(handler EventHandler) {
	el.lock.Lock()
	defer el.lock.Unlock()
	el.handlers = append(el.handlers, handler)
}

func (el *EventListener) Stop() {
	el.cancel()
}
