/*
Copyright 2019 The Knative Authors

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

package apiserver

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
)

type Adapter interface {
	Start(stopCh <-chan struct{}) error
}

type adapter struct {
	gvrs      []schema.GroupVersionResource
	k8s       dynamic.Interface
	ce        cloudevents.Client
	source    string
	namespace string
	logger    *zap.SugaredLogger
}

func NewAdaptor(source, namespace string, k8sClient dynamic.Interface, ceClient cloudevents.Client, logger *zap.SugaredLogger, gvr ...schema.GroupVersionResource) Adapter {
	a := &adapter{
		k8s:       k8sClient,
		ce:        ceClient,
		source:    source,
		namespace: namespace,
		gvrs:      gvr,
		logger:    logger,
	}
	return a
}

/*
TODO: No longer sending events for the controller of the updated object, a al:

	if controlled {
		informer.AddEventHandler(reconciler.Handler(impl.EnqueueControllerOf))
	} else {
		informer.AddEventHandler(reconciler.Handler(impl.Enqueue))
	}

*/

func (a *adapter) Start(stopCh <-chan struct{}) error {
	// Local stop channel.
	stop := make(chan struct{})

	factory := duck.TypedInformerFactory{
		Client:       a.k8s,
		ResyncPeriod: time.Duration(10 * time.Hour),
		StopChannel:  stop,
		Type:         &duckv1alpha1.KResource{},
	}

	for _, gvr := range a.gvrs {
		informer, _, err := factory.GetNamespaced(gvr, a.namespace)
		if err != nil {
			return err
		}

		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    a.addEvent,
			UpdateFunc: a.updateEvent,
			DeleteFunc: a.deleteEvent,
		})
	}

	<-stopCh
	stop <- struct{}{}
	return nil
}

func (a *adapter) addEvent(obj interface{}) {
	event, err := a.makeAddEvent(obj)
	if err != nil {
		a.logger.Info("event creation failed", zap.Error(err))
		return
	}

	if _, err := a.ce.Send(context.Background(), *event); err != nil {
		a.logger.Info("event delivery failed", zap.Error(err))
	}
}

func (a *adapter) updateEvent(oldObj, newObj interface{}) {
	event, err := a.makeUpdateEvent(oldObj, newObj)
	if err != nil {
		a.logger.Info("event creation failed", zap.Error(err))
		return
	}

	if _, err := a.ce.Send(context.Background(), *event); err != nil {
		a.logger.Info("event delivery failed", zap.Error(err))
	}
}

func (a *adapter) deleteEvent(obj interface{}) {
	event, err := a.makeDeleteEvent(obj)
	if err != nil {
		a.logger.Info("event creation failed", zap.Error(err))
		return
	}

	if _, err := a.ce.Send(context.Background(), *event); err != nil {
		a.logger.Info("event delivery failed", zap.Error(err))
	}
}
