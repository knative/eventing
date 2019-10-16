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
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/tracker"
)

// ListableTracker is a tracker capable of tracking any object that implements the apis.Listable interface.
type ListableTracker interface {
	// TrackInNamespace returns a function that can be used to watch arbitrary apis.Listable resources in the same
	// namespace as obj. Any change will cause a callback for obj.
	TrackInNamespace(obj metav1.Object) Track
	// ListerFor returns the lister for the object reference. It returns an error if the lister does not exist.
	ListerFor(ref corev1.ObjectReference) (cache.GenericLister, error)
	// InformerFor returns the informer for the object reference. It returns an error if the informer does not exist.
	InformerFor(ref corev1.ObjectReference) (cache.SharedIndexInformer, error)
}

type Track func(corev1.ObjectReference) error

// NewListableTracker creates a new ListableTracker, backed by a TypedInformerFactory.
func NewListableTracker(ctx context.Context, listable apis.Listable, callback func(types.NamespacedName), lease time.Duration) ListableTracker {
	return &listableTracker{
		informerFactory: &duck.TypedInformerFactory{
			Client:       dynamicclient.Get(ctx),
			Type:         listable,
			ResyncPeriod: controller.GetResyncPeriod(ctx),
			StopChannel:  ctx.Done(),
		},
		tracker:  tracker.New(callback, lease),
		concrete: map[schema.GroupVersionResource]informerListerPair{},
	}
}

type listableTracker struct {
	informerFactory duck.InformerFactory
	tracker         tracker.Interface

	concrete     map[schema.GroupVersionResource]informerListerPair
	concreteLock sync.RWMutex
}

type informerListerPair struct {
	informer cache.SharedIndexInformer
	lister   cache.GenericLister
}

// ensureTracking ensures that there is an informer watching and sending events to tracker for the
// concrete GVK. It also ensures that there is the corresponding lister for that informer.
func (t *listableTracker) ensureTracking(ref corev1.ObjectReference) error {
	if equality.Semantic.DeepEqual(ref, &corev1.ObjectReference{}) {
		return errors.New("cannot track empty object ref")
	}
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
	informer, lister, err := t.informerFactory.Get(gvr)
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
	t.concrete[gvr] = informerListerPair{informer: informer, lister: lister}
	return nil
}

// TrackInNamespace satisfies the ListableTracker interface.
func (t *listableTracker) TrackInNamespace(obj metav1.Object) Track {
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

// ListerFor satisfies the ListableTracker interface.
func (t *listableTracker) ListerFor(ref corev1.ObjectReference) (cache.GenericLister, error) {
	if equality.Semantic.DeepEqual(ref, &corev1.ObjectReference{}) {
		return nil, errors.New("cannot get lister for empty object ref")
	}
	gvk := ref.GroupVersionKind()
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)

	t.concreteLock.RLock()
	defer t.concreteLock.RUnlock()
	informerListerPair, present := t.concrete[gvr]
	if !present {
		return nil, fmt.Errorf("no lister available for GVR %s", gvr.String())
	}
	return informerListerPair.lister, nil
}

// InformerFor satisfies the ListableTracker interface.
func (t *listableTracker) InformerFor(ref corev1.ObjectReference) (cache.SharedIndexInformer, error) {
	if equality.Semantic.DeepEqual(ref, &corev1.ObjectReference{}) {
		return nil, errors.New("cannot get informer for empty object ref")
	}
	gvk := ref.GroupVersionKind()
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)

	t.concreteLock.RLock()
	defer t.concreteLock.RUnlock()
	informerListerPair, present := t.concrete[gvr]
	if !present {
		return nil, fmt.Errorf("no informer available for GVR %s", gvr.String())
	}
	return informerListerPair.informer, nil
}
