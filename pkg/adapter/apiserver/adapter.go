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
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"

	cloudevents "github.com/cloudevents/sdk-go"
	"go.uber.org/zap"
)

type Adapter interface {
	Start(stopCh <-chan struct{}) error
}

const (
	// RefMode produces payloads of ObjectReference
	RefMode = "Ref"
	// ResourceMode produces payloads of ResourceEvent
	ResourceMode = "Resource"
)

// Options hold the options for the Adapter.
type Options struct {
	Mode      string
	Namespace string
	GVRCs     []GVRC
}

// GVRC is a pairing of GroupVersionResource and Controller flag.
type GVRC struct {
	GVR        schema.GroupVersionResource
	Controller bool
}

type adapter struct {
	gvrcs     []GVRC
	k8s       dynamic.Interface
	ce        cloudevents.Client
	source    string
	namespace string
	logger    *zap.SugaredLogger

	mode     string
	delegate eventDelegate
}

func NewAdaptor(source string, k8sClient dynamic.Interface, ceClient cloudevents.Client, logger *zap.SugaredLogger, opt Options) Adapter {
	mode := opt.Mode
	switch mode {
	case ResourceMode, RefMode:
		// ok
	default:
		logger.Warn("unknown mode ", mode)
		mode = RefMode
		logger.Warn("defaulting mode to ", mode)
	}

	a := &adapter{
		k8s:       k8sClient,
		ce:        ceClient,
		source:    source,
		logger:    logger,
		gvrcs:     opt.GVRCs,
		namespace: opt.Namespace,
		mode:      mode,
	}
	return a
}

type eventDelegate interface {
	addEvent(obj interface{})
	updateEvent(oldObj, newObj interface{})
	deleteEvent(obj interface{})

	addControllerWatch(gvr schema.GroupVersionResource)
}

func (a *adapter) Start(stopCh <-chan struct{}) error {
	// Local stop channel.
	stop := make(chan struct{})

	resyncPeriod := time.Duration(10 * time.Hour)

	var d eventDelegate
	switch a.mode {
	case ResourceMode:
		d = &resource{
			ce:     a.ce,
			source: a.source,
			logger: a.logger,
		}

	case RefMode:
		d = &ref{
			ce:     a.ce,
			source: a.source,
			logger: a.logger,
		}

	default:
		return fmt.Errorf("mode %q not understood", a.mode)
	}

	for _, gvrc := range a.gvrcs {
		var informer cache.SharedIndexInformer

		lw := &cache.ListWatch{
			ListFunc:  asUnstructuredLister(a.k8s.Resource(gvrc.GVR).Namespace(a.namespace).List),
			WatchFunc: asUnstructuredWatcher(a.k8s.Resource(gvrc.GVR).Namespace(a.namespace).Watch),
		}
		informer = cache.NewSharedIndexInformer(lw, &unstructured.Unstructured{}, resyncPeriod, cache.Indexers{
			cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
		})

		go informer.Run(stopCh)

		if ok := cache.WaitForCacheSync(stopCh, informer.HasSynced); !ok {
			return fmt.Errorf("failed starting shared index informer for %s on namespace %s", gvrc.GVR.String(), a.namespace)
		}

		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    d.addEvent,
			UpdateFunc: d.updateEvent,
			DeleteFunc: d.deleteEvent,
		})
		if gvrc.Controller {
			d.addControllerWatch(gvrc.GVR)
		}
	}

	<-stopCh
	stop <- struct{}{}
	return nil
}

type unstructuredLister func(metav1.ListOptions) (*unstructured.UnstructuredList, error)

func asUnstructuredLister(ulist unstructuredLister) cache.ListFunc {
	return func(opts metav1.ListOptions) (runtime.Object, error) {
		ul, err := ulist(opts)
		if err != nil {
			return nil, err
		}
		return ul, nil
	}
}

func asUnstructuredWatcher(wf cache.WatchFunc) cache.WatchFunc {
	return func(lo metav1.ListOptions) (watch.Interface, error) {
		return wf(lo)
	}
}
