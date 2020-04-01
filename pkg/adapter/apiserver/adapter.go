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

package apiserver

import (
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
)

type envConfig struct {
	adapter.EnvConfig
	Name string `envconfig:"NAME" required:"true"`

	ConfigJson string `envconfig:"K_SOURCE_CONFIG" required:"true"`
}

type apiServerAdapter struct {
	ce     cloudevents.Client
	logger *zap.SugaredLogger

	config Config

	k8s    dynamic.Interface
	source string // TODO: who dis?
	name   string // TODO: who dis?
}

func (a *apiServerAdapter) Start(stopCh <-chan struct{}) error {
	// Local stop channel.
	stop := make(chan struct{})

	resyncPeriod := 10 * time.Hour

	var delegate cache.Store = &resourceDelegate{
		ce:     a.ce,
		source: a.source,
		logger: a.logger,
		ref:    a.config.EventMode == v1alpha2.ReferenceMode,
	}

	if a.config.ResourceOwner != nil {
		if a.config.ResourceOwner.APIVersion != nil && a.config.ResourceOwner.Kind != nil {
			a.logger.Infow("will be filtered",
				zap.String("APIVersion", *a.config.ResourceOwner.APIVersion),
				zap.String("Kind", *a.config.ResourceOwner.Kind))
			delegate = &controllerFilter{
				apiVersion: *a.config.ResourceOwner.APIVersion,
				kind:       *a.config.ResourceOwner.Kind,
				delegate:   delegate,
			}
		}
	}

	a.logger.Infof("STARTING -- %#v", a.config)

	for _, r := range a.config.Resources {
		lw := &cache.ListWatch{
			// TODO: this will not work with cluster scoped resources.
			ListFunc:  asUnstructuredLister(a.k8s.Resource(r.GVR).Namespace(a.config.Namespace).List, r.LabelSelector),
			WatchFunc: asUnstructuredWatcher(a.k8s.Resource(r.GVR).Namespace(a.config.Namespace).Watch, r.LabelSelector),
		}

		reflector := cache.NewReflector(lw, &unstructured.Unstructured{}, delegate, resyncPeriod)
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
