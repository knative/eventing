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

const (
	// MaxNamespacesForIndividualWatches is the threshold above which we should consider
	// issue a warning to avoid performance issues with many individual watches.
	MaxNamespacesForIndividualWatches = 100
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

type resourceWatchMatch struct {
	// resourceWatch is the target configuration for the resource watch.
	resourceWatch *ResourceWatch
	// apiResource is the API resource information matched to the resource watch.
	apiResource *metav1.APIResource
	// resourceInterfaces are the dynamic interfaces that are matched to the api resource and namespace configuration in the resource watch.
	resourceInterfaces []dynamic.ResourceInterface
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

	// we have two modes of operation for the ApiServerSource adapter:
	// 1. Resilient Mode (Default): The adapter uses `reflector.Run()` to continuously retry establishing watches
	//    on resources, making it resilient to transient errors or delayed permission grants.
	// 2. Fail-Fast Mode: In this mode, the adapter uses `reflector.ListAndWatchWithContext()`. If any resource watch
	//    fails to be established on the first attempt, the entire adapter will fail immediately. This provides faster
	//    feedback and a clearer failure state in environments where permissions are expected to be correct at startup.

	if a.config.FailFast {
		a.logger.Info("Starting in fail-fast mode. Any single watch failure will stop the adapter.")
		return a.startFailFast(ctx, stopCh, delegate, matches)
	}
	a.logger.Info("Starting in resilient mode. Watch failures will be retried.")
	return a.startResilient(ctx, stopCh, delegate, matches)
}

func (a *apiServerAdapter) startResilient(ctx context.Context, stopCh <-chan struct{}, delegate cache.Store, matches []resourceWatchMatch) error {
	// Local stop channel.
	stop := make(chan struct{})

	resyncPeriod := 10 * time.Hour

	a.logger.Infof("STARTING -- %#v", a.config)

	for _, match := range matches {
		if match.apiResource == nil {
			a.logger.Errorf("could not retrieve information about resource %s: it doesn't exist. skipping...", match.resourceWatch.GVR.String())
			// This was already logged as a warning in collectResourceMatches.
			// In resilient mode, we just skip it and continue.
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

// startFailFast starts the ApiServerSource in fail fast mode, where it attempts to create watches only once.
// If any single watch fails, it will return an error and stop all watches.
// This would result in the APIServerSource status marked as failed.
func (a *apiServerAdapter) startFailFast(ctx context.Context, stopCh <-chan struct{}, delegate cache.Store, matches []resourceWatchMatch) error {
	resyncPeriod := 10 * time.Hour

	a.logger.Infof("STARTING -- %#v", a.config)
	a.logger.Info("ApiServerSource is in fail fast mode, so watch creation will be attempted only once")

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

func (a *apiServerAdapter) collectResourceMatches() ([]resourceWatchMatch, error) {
	matches := make([]resourceWatchMatch, 0, len(a.config.Resources))

	for _, configRes := range a.config.Resources {
		resources, err := a.discover.ServerResourcesForGroupVersion(configRes.GVR.GroupVersion().String())
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve information about resource %s: %v", configRes.GVR.String(), err)
		}

		match := resourceWatchMatch{
			resourceWatch:      &configRes,
			apiResource:        nil,
			resourceInterfaces: make([]dynamic.ResourceInterface, 0),
		}
		for _, apires := range resources.APIResources {
			if apires.Name == configRes.GVR.Resource {
				match.apiResource = &apires

				if apires.Namespaced && !a.config.AllNamespaces {
					// Warn when watching many namespaces as this can cause performance issues
					// See: https://github.com/knative/eventing/issues/8675
					if len(a.config.Namespaces) > MaxNamespacesForIndividualWatches {
						a.logger.Warnf("ApiServerSource is watching %d namespaces for resource %s, which exceeds the recommended threshold of %d. "+
							"This may cause performance issues and watch connection failures. "+
							"Consider using cluster-wide watches with client-side filtering for better performance.",
							len(a.config.Namespaces), configRes.GVR.String(), MaxNamespacesForIndividualWatches)
					}
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
