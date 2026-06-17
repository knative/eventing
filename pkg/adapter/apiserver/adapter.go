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
	"math"
	"sync"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
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

	discover  discovery.DiscoveryInterface
	k8s       dynamic.Interface
	source    string // TODO: who dis?
	name      string // TODO: who dis?
	namespace string
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

	// we have three modes of operation for the ApiServerSource adapter:
	// 1. No-Cache Mode: The adapter skips the initial LIST call and only watches for new events.
	//    This reduces API server load significantly when watching across many namespaces.
	//    Pre-existing objects will not emit events on startup.
	// 2. Resilient Mode (Default): The adapter uses `reflector.Run()` to continuously retry establishing watches
	//    on resources, making it resilient to transient errors or delayed permission grants.
	// 3. Fail-Fast Mode: In this mode, the adapter uses `reflector.ListAndWatchWithContext()`. If any resource watch
	//    fails to be established on the first attempt, the entire adapter will fail immediately. This provides faster
	//    feedback and a clearer failure state in environments where permissions are expected to be correct at startup.

	if a.config.DisableCache {
		a.logger.Info("Starting in no-cache mode. Initial LIST is skipped; only new events will be emitted.")
		return a.startWatchOnly(ctx, stopCh, delegate, matches)
	}
	if a.config.FailFast {
		a.logger.Info("Starting in fail-fast mode. Any single watch failure will stop the adapter.")
		return a.startFailFast(ctx, stopCh, delegate, matches)
	}
	a.logger.Info("Starting in resilient mode. Watch failures will be retried.")
	return a.startResilient(ctx, stopCh, delegate, matches)
}

// startWatchOnly starts watches for all matched resources. It performs a lightweight
// LIST (limit=1) per resource interface to obtain the current resourceVersion, then
// issues a Watch from that point. This means pre-existing objects do not produce
// events on startup, and startup API load is O(resources*namespaces) lightweight
// LISTs rather than full object dumps.
//
// When both DisableCache and FailFast are set, DisableCache takes precedence.
func (a *apiServerAdapter) startWatchOnly(ctx context.Context, stopCh <-chan struct{}, delegate cache.Store, matches []resourceWatchMatch) error {
	if a.config.FailFast {
		a.logger.Warn("disableCache=true takes precedence over failFast=true; running in no-cache mode without fail-fast behavior")
	}

	watchCtx, cancelWatchers := context.WithCancel(ctx)
	defer cancelWatchers()

	var wg sync.WaitGroup
	for _, match := range matches {
		if match.apiResource == nil {
			a.logger.Errorf("could not retrieve information about resource %s: it doesn't exist. skipping...", match.resourceWatch.GVR.String())
			continue
		}
		for _, res := range match.resourceInterfaces {
			wg.Add(1)
			go func() {
				defer wg.Done()
				a.watchResourceLoop(watchCtx, res, match.resourceWatch.LabelSelector, delegate)
			}()
		}
	}

	select {
	case <-stopCh:
	case <-ctx.Done():
	}
	cancelWatchers()
	wg.Wait()
	return nil
}

func noCacheBackoff() wait.Backoff {
	return wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
		Steps:    math.MaxInt32,
		Cap:      60 * time.Second,
	}
}

// watchResourceLoop runs a continuous List+Watch loop for a single resource interface.
// A lightweight LIST (limit=1) is issued before each Watch to obtain the current
// resourceVersion so that Watch does not replay synthetic ADDED events for
// pre-existing objects (the behaviour when resourceVersion="" is passed to Watch).
// On watch.Error with StatusReasonGone (HTTP 410 — resourceVersion expired from the
// watch cache), the loop re-lists to get a fresh resourceVersion.
// All failures back off exponentially with jitter before retrying.
func (a *apiServerAdapter) watchResourceLoop(ctx context.Context, ri dynamic.ResourceInterface, labelSelector string, delegate cache.Store) {
	backoff := noCacheBackoff()
	for ctx.Err() == nil {
		list, err := ri.List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
			Limit:         1,
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			a.logger.Errorw("failed to list for resourceVersion, retrying", zap.Error(err))
			t := time.NewTimer(backoff.Step())
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}
			continue
		}
		rv := list.GetResourceVersion()
		if rv == "" {
			a.logger.Warn("LIST returned empty resourceVersion, will retry after backoff")
			t := time.NewTimer(backoff.Step())
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}
			continue
		}

		timeout := int64(5 * 60)
		w, err := ri.Watch(ctx, metav1.ListOptions{
			LabelSelector:   labelSelector,
			ResourceVersion: rv,
			TimeoutSeconds:  &timeout,
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			a.logger.Errorw("watch error, retrying", zap.Error(err))
			t := time.NewTimer(backoff.Step())
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}
			continue
		}

		watchStart := time.Now()
		a.drainWatchEvents(ctx, w, delegate)
		if time.Since(watchStart) > 30*time.Second {
			// Watch was long-lived; treat connection as stable and reset backoff.
			backoff = noCacheBackoff()
		} else {
			t := time.NewTimer(backoff.Step())
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}
		}
	}
}

// drainWatchEvents reads from a watch until the context is done, the watch
// channel closes, or a watch.Error event is received. On StatusReasonGone
// (HTTP 410), it returns immediately so watchResourceLoop can re-list.
func (a *apiServerAdapter) drainWatchEvents(ctx context.Context, w watch.Interface, delegate cache.Store) {
	defer w.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.ResultChan():
			if !ok {
				return
			}
			switch event.Type {
			case watch.Added:
				if obj, ok := event.Object.(*unstructured.Unstructured); ok {
					if err := delegate.Add(obj); err != nil {
						a.logger.Errorw("failed to add object to delegate store", zap.Error(err))
					}
				}
			case watch.Modified:
				if obj, ok := event.Object.(*unstructured.Unstructured); ok {
					if err := delegate.Update(obj); err != nil {
						a.logger.Errorw("failed to update object in delegate store", zap.Error(err))
					}
				}
			case watch.Deleted:
				if obj, ok := event.Object.(*unstructured.Unstructured); ok {
					if err := delegate.Delete(obj); err != nil {
						a.logger.Errorw("failed to delete object from delegate store", zap.Error(err))
					}
				}
			case watch.Error:
				if status, ok := event.Object.(*metav1.Status); ok && status.Reason == metav1.StatusReasonGone {
					a.logger.Info("watch resourceVersion expired (410 Gone), will re-list")
				} else {
					a.logger.Errorw("received watch error event", zap.Any("object", event.Object))
				}
				return
			}
		}
	}
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
		for i, res := range match.resourceInterfaces {
			lw := &cache.ListWatch{
				ListFunc:  asUnstructuredLister(ctx, res.List, match.resourceWatch.LabelSelector),
				WatchFunc: asUnstructuredWatcher(ctx, res.Watch, match.resourceWatch.LabelSelector),
			}

			reflectorName := a.buildReflectorName(match.apiResource.Namespaced, match.resourceWatch.GVR.String(), i)
			reflector := cache.NewNamedReflector(reflectorName, lw, &unstructured.Unstructured{}, delegate, resyncPeriod)
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
		for i, res := range match.resourceInterfaces {
			lw := &cache.ListWatch{
				ListFunc:  asUnstructuredLister(watchCtx, res.List, match.resourceWatch.LabelSelector),
				WatchFunc: asUnstructuredWatcher(watchCtx, res.Watch, match.resourceWatch.LabelSelector),
			}

			reflectorName := a.buildReflectorName(match.apiResource.Namespaced, match.resourceWatch.GVR.String(), i)
			reflector := cache.NewNamedReflector(reflectorName, lw, &unstructured.Unstructured{}, delegate, resyncPeriod)
			wg.Add(1)
			go func() {
				defer wg.Done()
				// ListAndWatch could exit with `nil` under some circumstances, it shouldn't
				// ever stop listening and watching, so we will treat any exit as a reason to restart
				err := reflector.ListAndWatchWithContext(watchCtx)
				a.logger.Error("reflector exited: %v", err)

				select {
				case errorChan <- fmt.Errorf("failed to watch: %v", err):
				default:
				}
				cancelWatchers()
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
			cancelWatchers()
			wg.Wait()
			a.logger.Errorf("ApiServerSource failed: %v", err)
			return err
		}
	case <-stopCh:
		cancelWatchers()
		wg.Wait()
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
		apiServerSourceNS:   a.namespace,
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

// buildReflectorName builds the reflector name with resource GVR and namespace.
// For namespaced resources (when not watching all namespaces), the name includes
// both the GVR and the specific namespace being watched (e.g., "apps/v1/deployments/default").
// For cluster-scoped resources or when watching all namespaces, only the GVR is used.
func (a *apiServerAdapter) buildReflectorName(namespaced bool, gvr string, namespaceIndex int) string {
	if namespaced && !a.config.AllNamespaces {
		return fmt.Sprintf("%s/%s", gvr, a.config.Namespaces[namespaceIndex])
	}
	return gvr
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
