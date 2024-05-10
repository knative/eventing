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
	"fmt"
	"net/http"
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
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	brokerfilter "knative.dev/eventing/pkg/broker/filter"
	"knative.dev/eventing/pkg/eventfilter/subscriptionsapi"
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
		ce:                  a.ce,
		source:              a.source,
		logger:              a.logger,
		ref:                 a.config.EventMode == v1.ReferenceMode,
		apiServerSourceName: a.name,
		filter:              subscriptionsapi.NewAllFilter(brokerfilter.MaterializeFiltersList(a.logger.Desugar(), a.config.Filters)...),
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
			return fmt.Errorf("failed to retrieve information about resource %s: %v", configRes.GVR.String(), err)
		}

		exists := false
		for _, apires := range resources.APIResources {
			if apires.Name == configRes.GVR.Resource {
				var resources []dynamic.ResourceInterface
				if apires.Namespaced && !a.config.AllNamespaces {
					for _, ns := range a.config.Namespaces {
						resources = append(resources, a.k8s.Resource(configRes.GVR).Namespace(ns))
					}
				} else {
					resources = append(resources, a.k8s.Resource(configRes.GVR))
				}

				for _, res := range resources {
					lw := &cache.ListWatch{
						ListFunc:  asUnstructuredLister(ctx, res.List, configRes.LabelSelector),
						WatchFunc: asUnstructuredWatcher(ctx, res.Watch, configRes.LabelSelector),
					}

					reflector := cache.NewReflector(lw, &unstructured.Unstructured{}, delegate, resyncPeriod)
					go reflector.Run(stop)
				}

				exists = true
				break
			}
		}

		if !exists {
			a.logger.Errorf("could not retrieve information about resource %s: it doesn't exist", configRes.GVR.String())
		}
	}

	srv := &http.Server{
		Addr: ":8080",
		// Configure read header timeout to overcome potential Slowloris Attack because ReadHeaderTimeout is not
		// configured in the http.Server.
		ReadHeaderTimeout: 10 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}
	go srv.ListenAndServe()

	<-stopCh
	stop <- struct{}{}
	srv.Shutdown(ctx)
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
