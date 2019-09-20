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

package duck

import (
	"context"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis/duck"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/tracker"
)

// ResourceInformer is an informer that allows tracking arbitrary Resources.
type ResourceInformer interface {
	NewTracker(callback func(types.NamespacedName), lease time.Duration) ResourceTracker
}

// ResourceTracker is a tracker capable of tracking Resources.
type ResourceTracker interface {
	// TrackInNamespace returns a function that can be used to watch arbitrary Resources in the same
	// namespace as obj. Any change will cause a callback for obj.
	TrackInNamespace(obj metav1.Object) func(corev1.ObjectReference) error
}

// resourceInformer is a concrete implementation of ResourceInformer. It caches informers and
// ensures TypeMeta.
type resourceInformer struct {
	duck duck.InformerFactory

	concrete     map[schema.GroupVersionResource]cache.SharedIndexInformer
	concreteLock sync.RWMutex
}

// NewResourceInformer creates a new ResourceInformer.
func NewResourceInformer(ctx context.Context) ResourceInformer {
	return &resourceInformer{
		duck: &duck.TypedInformerFactory{
			Client:       dynamicclient.Get(ctx),
			Type:         &duckv1alpha1.Resource{},
			ResyncPeriod: controller.GetResyncPeriod(ctx),
			StopChannel:  ctx.Done(),
		},
		concrete: map[schema.GroupVersionResource]cache.SharedIndexInformer{},
	}
}

// NewTracker creates a new ResourceTracker, backed by this ResourceInformer.
func (i *resourceInformer) NewTracker(callback func(types.NamespacedName), lease time.Duration) ResourceTracker {
	return &resourceTracker{
		informer: i,
		tracker:  tracker.New(callback, lease),
		concrete: map[schema.GroupVersionResource]struct{}{},
	}
}

// ensureInformer ensures that there is an informer watching on a concrete GVK.
func (i *resourceInformer) ensureInformer(ref corev1.ObjectReference) (cache.SharedIndexInformer, error) {
	gvk := ref.GroupVersionKind()
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)

	i.concreteLock.RLock()
	informer, present := i.concrete[gvr]
	i.concreteLock.RUnlock()
	if present {
		// There is already an informer running for this GVR, we don't need or want to make
		// a second one.
		return informer, nil
	}
	// There is not an informer for this GVR, make one.
	i.concreteLock.Lock()
	defer i.concreteLock.Unlock()
	// Now that we have acquired the write lock, make sure no one has made the informer.
	if informer, present = i.concrete[gvr]; present {
		return informer, nil
	}
	informer, _, err := i.duck.Get(gvr)
	if err != nil {
		return nil, err
	}
	i.concrete[gvr] = informer
	return informer, nil
}

type resourceTracker struct {
	informer *resourceInformer
	tracker  tracker.Interface

	concrete     map[schema.GroupVersionResource]struct{}
	concreteLock sync.RWMutex
}

// ensureInformer ensures that there is an informer watching and sending events to tracker for the
// concrete GVK.
func (t *resourceTracker) ensureTracking(ref corev1.ObjectReference) error {
	gvk := ref.GroupVersionKind()
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)

	t.concreteLock.RLock()
	_, present := t.concrete[gvr]
	t.concreteLock.RUnlock()
	if present {
		// There is already an informer running for this GVR, we don't need or want to make
		// a second one.
		return nil
	}
	// Tracking has not been setup for this GVR.
	t.concreteLock.Lock()
	defer t.concreteLock.Unlock()
	// Now that we have acquired the write lock, make sure no one has added tracking handlers.
	if _, present = t.concrete[gvr]; present {
		return nil
	}
	informer, err := t.informer.ensureInformer(ref)
	if err != nil {
		return err
	}
	informer.AddEventHandler(controller.HandleAll(
		// Call the tracker's OnChanged method, but we've seen the objects coming through
		// this path missing TypeMeta, so ensure it is properly populated.
		controller.EnsureTypeMeta(
			t.tracker.OnChanged,
			gvk,
		),
	))
	t.concrete[gvr] = struct{}{}
	return nil
}

// TrackInNamespace satisfies the ResourceTracker interface.
func (t *resourceTracker) TrackInNamespace(obj metav1.Object) func(corev1.ObjectReference) error {
	return func(ref corev1.ObjectReference) error {
		// This is often used by Trigger and Subscription, both of which pass in refs that do not
		// specify the namespace.
		ref.Namespace = obj.GetNamespace()
		if err := t.ensureTracking(ref); err != nil {
			return err
		}
		return t.tracker.Track(ref, obj)
	}
}
