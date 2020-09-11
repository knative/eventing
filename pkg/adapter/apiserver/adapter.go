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
	"context"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
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

	discover discovery.DiscoveryInterface
	k8s      dynamic.Interface
	source   string // TODO: who dis?
	name     string // TODO: who dis?
}

func (a *apiServerAdapter) Start(ctx context.Context) error {
	return a.start(ctx, ctx.Done())
}

func (a *apiServerAdapter) start(ctx context.Context, stopCh <-chan struct{}) error {
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
		a.logger.Infow("will be filtered",
			zap.String("APIVersion", a.config.ResourceOwner.APIVersion),
			zap.String("Kind", a.config.ResourceOwner.Kind))
		delegate = &controllerFilter{
			apiVersion: a.config.ResourceOwner.APIVersion,
			kind:       a.config.ResourceOwner.Kind,
			delegate:   delegate,
		}
	}

	a.logger.Infof("STARTING -- %#v", a.config)

	for _, configRes := range a.config.Resources {

		resources, err := a.discover.ServerResourcesForGroupVersion(configRes.GVR.GroupVersion().String())
		if err != nil {
			a.logger.Errorf("Could not retrieve information about resource %s: %s", configRes.GVR.String(), err.Error())
			continue
		}

		exists := false
		for _, apires := range resources.APIResources {
			if apires.Name == configRes.GVR.Resource {

				var res dynamic.ResourceInterface
				if apires.Namespaced {
					res = a.k8s.Resource(configRes.GVR).Namespace(a.config.Namespace)
				} else {
					res = a.k8s.Resource(configRes.GVR)
				}

				lw := &cache.ListWatch{
					ListFunc:  asUnstructuredLister(ctx, res.List, configRes.LabelSelector),
					WatchFunc: asUnstructuredWatcher(ctx, res.Watch, configRes.LabelSelector),
				}

				reflector := cache.NewReflector(lw, &unstructured.Unstructured{}, delegate, resyncPeriod)
				go reflector.Run(stop)
				exists = true
				break
			}
		}

		if !exists {
			a.logger.Errorf("Could not retrieve information about resource %s: %s", configRes.GVR.String())
		}
	}

	<-stopCh
	stop <- struct{}{}
	return nil
}

type unstructuredLister func(context.Context, metav1.ListOptions) (*unstructured.UnstructuredList, error)

func asUnstructuredLister(ctx context.Context, ulist unstructuredLister, selector string) cache.ListFunc {
	return func(opts metav1.ListOptions) (runtime.Object, error) {
		if selector != "" && opts.LabelSelector == "" {
			opts.LabelSelector = selector
		}
		ul, err := ulist(ctx, opts)
		if err != nil {
			return nil, err
		}
		return ul, nil
	}
}

type structuredWatcher func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)

func asUnstructuredWatcher(ctx context.Context, wf structuredWatcher, selector string) cache.WatchFunc {
	return func(lo metav1.ListOptions) (watch.Interface, error) {
		if selector != "" && lo.LabelSelector == "" {
			lo.LabelSelector = selector
		}
		return wf(ctx, lo)
	}
}
