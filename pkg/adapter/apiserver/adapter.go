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

	cloudevents "github.com/cloudevents/sdk-go"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/source"
)

type Adapter interface {
	Start(stopCh <-chan struct{}) error
}

const (
	// RefMode produces payloads of ObjectReference
	RefMode = "Ref"
	// ResourceMode produces payloads of ResourceEvent
	ResourceMode = "Resource"

	resourceGroup = "apiserversources.sources.eventing.knative.dev"
)

// Options hold the options for the Adapter.
type Options struct {
	Mode      string
	Namespace string
	GVRCs     []GVRC
}

// GVRC is a combination of GroupVersionResource, Controller flag and LabelSelector.
type GVRC struct {
	GVR           schema.GroupVersionResource
	Controller    bool
	LabelSelector string
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
	reporter source.StatsReporter
	name     string
}

func NewAdaptor(source string, k8sClient dynamic.Interface,
	ceClient cloudevents.Client, logger *zap.SugaredLogger,
	opt Options, reporter source.StatsReporter, name string) Adapter {
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
		reporter:  reporter,
		name:      name,
	}
	return a
}

type eventDelegate interface {
	cache.Store
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
			ce:        a.ce,
			source:    a.source,
			logger:    a.logger,
			reporter:  a.reporter,
			namespace: a.namespace,
			name:      a.name,
		}

	case RefMode:
		d = &ref{
			ce:        a.ce,
			source:    a.source,
			logger:    a.logger,
			reporter:  a.reporter,
			namespace: a.namespace,
			name:      a.name,
		}

	default:
		return fmt.Errorf("mode %q not understood", a.mode)
	}

	for _, gvrc := range a.gvrcs {
		lw := &cache.ListWatch{
			ListFunc:  asUnstructuredLister(a.k8s.Resource(gvrc.GVR).Namespace(a.namespace).List, gvrc.LabelSelector),
			WatchFunc: asUnstructuredWatcher(a.k8s.Resource(gvrc.GVR).Namespace(a.namespace).Watch, gvrc.LabelSelector),
		}

		if gvrc.Controller {
			d.addControllerWatch(gvrc.GVR)
		}

		reflector := cache.NewReflector(lw, &unstructured.Unstructured{}, d, resyncPeriod)
		go reflector.Run(stop)
	}

	<-stopCh
	stop <- struct{}{}
	return nil
}

type unstructuredLister func(metav1.ListOptions) (*unstructured.UnstructuredList, error)

func asUnstructuredLister(ulist unstructuredLister, selector string) cache.ListFunc {
	return func(opts metav1.ListOptions) (runtime.Object, error) {
		if selector != "" && opts.LabelSelector == "" {
			opts.LabelSelector = selector
		}
		ul, err := ulist(opts)
		if err != nil {
			return nil, err
		}
		return ul, nil
	}
}

func asUnstructuredWatcher(wf cache.WatchFunc, selector string) cache.WatchFunc {
	return func(lo metav1.ListOptions) (watch.Interface, error) {
		if selector != "" && lo.LabelSelector == "" {
			lo.LabelSelector = selector
		}
		return wf(lo)
	}
}
