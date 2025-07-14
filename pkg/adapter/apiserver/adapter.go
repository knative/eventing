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
	"sync"
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
	delegate := a.setupDelegate()

	a.logger.Infof("STARTING -- %#v", a.config)

	matches, err := a.collectResourceMatches()
	if err != nil {
		return fmt.Errorf("failed to collect resource matches: %v", err)
	}

	if a.config.SkippedPermissions {
		a.logger.Info("ApiServerSource skipped checking permissions so watches will be attempted only once")
		return a.startWithoutPermissionCheck(ctx, stopCh, delegate, matches)
	}
	return a.startWithPermissionCheck(ctx, stopCh, delegate, matches)
}

func (a *apiServerAdapter) setupDelegate() cache.Store {
	var delegate cache.Store = &resourceDelegate{
		ce:                  a.ce,
		source:              a.source,
		logger:              a.logger,
		ref:                 a.config.EventMode == v1.ReferenceMode,
		apiServerSourceName: a.name,
		filter:              subscriptionsapi.NewAllFilter(subscriptionsapi.MaterializeFiltersList(a.logger.Desugar(), a.config.Filters)...),
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
	return delegate
}

type ResourceWatchMatch struct {
	// resourceWatch is the target configuration for the resource watch.
	resourceWatch *ResourceWatch
	// apiResource is the API resource information matched to the resource watch.
	apiResource *metav1.APIResource
	// resourceInterfaces are the dynamic interfaces that are matched to the api resource and namespace configuration in the resource watch.
	resourceInterfaces []dynamic.ResourceInterface
}

func (a *apiServerAdapter) collectResourceMatches() ([]ResourceWatchMatch, error) {
	matches := make([]ResourceWatchMatch, 0, len(a.config.Resources))

	for _, configRes := range a.config.Resources {
		resources, err := a.discover.ServerResourcesForGroupVersion(configRes.GVR.GroupVersion().String())
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve information about resource %s: %v", configRes.GVR.String(), err)
		}

		match := ResourceWatchMatch{
			resourceWatch:      &configRes,
			apiResource:        nil,
			resourceInterfaces: make([]dynamic.ResourceInterface, 0),
		}
		for _, apires := range resources.APIResources {
			if apires.Name == configRes.GVR.Resource {
				match.apiResource = &apires

				if apires.Namespaced && !a.config.AllNamespaces {
					for _, ns := range a.config.Namespaces {
						match.resourceInterfaces = append(match.resourceInterfaces, a.k8s.Resource(configRes.GVR).Namespace(ns))
					}
				} else {
					match.resourceInterfaces = append(match.resourceInterfaces, a.k8s.Resource(configRes.GVR))
				}

				break
			}
		}

		matches = append(matches, match)

		if match.apiResource == nil {
			a.logger.Errorf("could not retrieve information about resource %s: it doesn't exist", configRes.GVR.String())
		}
	}

	return matches, nil
}

func (a *apiServerAdapter) startWithPermissionCheck(ctx context.Context, stopCh <-chan struct{}, delegate cache.Store, matches []ResourceWatchMatch) error {
	// Local stop channel.
	stop := make(chan struct{})

	resyncPeriod := 10 * time.Hour

	a.logger.Infof("STARTING -- %#v", a.config)

	for _, match := range matches {
		if match.apiResource == nil {
			a.logger.Errorf("could not retrieve information about resource %s: it doesn't exist. skipping...", match.resourceWatch.GVR.String())
			continue
		}
		for _, res := range match.resourceInterfaces {
			lw := &cache.ListWatch{
				ListFunc:  asUnstructuredLister(ctx, res.List, match.resourceWatch.LabelSelector),
				WatchFunc: asUnstructuredWatcher(ctx, res.Watch, match.resourceWatch.LabelSelector),
			}

			reflector := cache.NewReflector(lw, &unstructured.Unstructured{}, delegate, resyncPeriod)
			go reflector.Run(stop)
		}
	}

	<-stopCh
	close(stop)
	return nil
}

func (a *apiServerAdapter) startWithoutPermissionCheck(ctx context.Context, stopCh <-chan struct{}, delegate cache.Store, matches []ResourceWatchMatch) error {
	resyncPeriod := 10 * time.Hour

	a.logger.Infof("STARTING -- %#v", a.config)
	a.logger.Info("ApiServerSource skipped checking permissions so watches will be attempted only once")

	watchCtx, cancelWatchers := context.WithCancel(ctx)
	defer cancelWatchers()

	var wg sync.WaitGroup
	errorChan := make(chan error, 1)

	for _, match := range matches {
		if match.apiResource == nil {
			a.logger.Errorf("could not retrieve information about resource %s: it doesn't exist.", match.resourceWatch.GVR.String())
			return fmt.Errorf("resource %s does not exist", match.resourceWatch.GVR.String())
		}
		for _, res := range match.resourceInterfaces {
			lw := &cache.ListWatch{
				ListFunc:  asUnstructuredLister(watchCtx, res.List, match.resourceWatch.LabelSelector),
				WatchFunc: asUnstructuredWatcher(watchCtx, res.Watch, match.resourceWatch.LabelSelector),
			}

			reflector := cache.NewReflector(lw, &unstructured.Unstructured{}, delegate, resyncPeriod)
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := reflector.ListAndWatchWithContext(watchCtx); err != nil {
					a.logger.Errorf("reflector failed: %v", err)
					select {
					case errorChan <- fmt.Errorf("failed to watch: %v", err):
					default:
					}
					cancelWatchers()
				}
			}()
		}
	}

	go func() {
		wg.Wait()
		close(errorChan)
	}()

	select {
	case err := <-errorChan:
		if err != nil {
			a.logger.Errorf("ApiServerSource failed: %v", err)
			return err
		}
	case <-stopCh:
		cancelWatchers()
		return nil
	}

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
