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

package adapter

import (
	"context"
	"fmt"
	"log"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"

	"knative.dev/eventing/pkg/utils/cache"
)

type enableConfigMapWatcherKey struct{}

// WithConfigMapWatcherEnabled signals to MainWithContext that it should
// create and configure a ConfigMap watcher
func WithConfigMapWatcherEnabled(ctx context.Context) context.Context {
	return context.WithValue(ctx, enableConfigMapWatcherKey{}, struct{}{})
}

// IsConfigMapWatcherEnabled tells MainWithContext that the ConfigMapWatcher
// is enabled
func IsConfigMapWatcherEnabled(ctx context.Context) bool {
	val := ctx.Value(enableConfigMapWatcherKey{})
	return val != nil
}

type configMapWatcherKey struct{}

// WithConfigWatcher signals to MainWithContext that it should
// create and configure a ConfigMap watcher
func WithConfigMapWatcher(ctx context.Context, watcher configmap.Watcher) context.Context {
	return context.WithValue(ctx, configMapWatcherKey{}, watcher)
}

// ConfigMapWatcherFromContext gets the ConfigMapWatchers from the context
// or nil if there is none suitable
func ConfigMapWatcherFromContext(ctx context.Context) configmap.Watcher {
	val := ctx.Value(configMapWatcherKey{})
	if val == nil {
		return nil
	}
	if watcher, ok := val.(configmap.Watcher); ok {
		return watcher
	}
	return nil
}

// SetupConfigMapWatchOrDie establishes a watch of the configmaps in the component
// namespace that are labeled to be watched or dies by calling log.Fatalf.
func SetupConfigMapWatchOrDie(ctx context.Context, component string, namespace string) configmap.Watcher {
	kc := kubeclient.Get(ctx)
	// Create ConfigMaps watcher with label-based filter.
	cmLabelReqs, err := FilterConfigByLabelEquals(cache.ComponentLabelKey, component)
	if err != nil {
		log.Fatalf("Failed to generate requirement for label %s: %v", cache.ComponentLabelKey, err)
	}

	return configmap.NewInformedWatcher(kc, namespace, *cmLabelReqs)
}

// FilterConfigByLabelEquals returns an "equas" label requirement for the
// given label key and value
func FilterConfigByLabelEquals(labelKey string, labelValue string) (*labels.Requirement, error) {
	req, err := labels.NewRequirement(labelKey, selection.Equals, []string{labelValue})
	if err != nil {
		return nil, fmt.Errorf("could not construct label requirement: %w", err)
	}
	return req, nil
}
