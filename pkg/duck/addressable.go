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
	"sync"

	"github.com/knative/pkg/apis/duck/v1alpha1"

	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/apis/duck"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/tracker"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// AddressableInformer is an informer that allows tracking arbitrary Addressables.
type AddressableInformer struct {
	duck duck.InformerFactory

	concrete     map[schema.GroupVersionResource]struct{}
	concreteLock sync.RWMutex
}

func NewAddressableInformer(opt reconciler.Options) *AddressableInformer {
	return &AddressableInformer{
		duck: &duck.TypedInformerFactory{
			Client:       opt.DynamicClientSet,
			Type:         &v1alpha1.AddressableType{},
			ResyncPeriod: opt.ResyncPeriod,
			StopChannel:  opt.StopChannel,
		},
		concrete: map[schema.GroupVersionResource]struct{}{},
	}
}

// TrackInNamespace returns a function that can be used to watch arbitrary Addressables in the same
// namespace as obj. Any change will cause a callback for obj.
func (i *AddressableInformer) TrackInNamespace(tracker tracker.Interface, obj metav1.Object) func(corev1.ObjectReference) error {
	return func(ref corev1.ObjectReference) error {
		if err := i.ensureInformer(tracker, ref); err != nil {
			return err
		}

		// This is often used by Trigger and Subscription, both of which pass in refs that do not
		// specify the namespace, they only
		ref.Namespace = obj.GetNamespace()
		if err := tracker.Track(ref, obj); err != nil {
			return err
		}
		return nil
	}
}

// ensureInformer ensures that there is an informer watching and sending events to tracker for the
// concrete GVK.
func (i *AddressableInformer) ensureInformer(tracker tracker.Interface, ref corev1.ObjectReference) error {
	gvk := ref.GroupVersionKind()
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)

	i.concreteLock.RLock()
	_, present := i.concrete[gvr]
	i.concreteLock.RUnlock()
	if present {
		// There is already an informer running for this GVR, we don't need or want to make
		// a second one.
		return nil
	}
	// There is not an informer for this GVR, make one.
	i.concreteLock.Lock()
	defer i.concreteLock.Unlock()
	// Now that we have acquired the write lock, make sure no one has made the informer.
	if _, present = i.concrete[gvr]; present {
		return nil
	}
	informer, _, err := i.duck.Get(gvr)
	if err != nil {
		return err
	}
	informer.AddEventHandler(reconciler.Handler(
		// Call the tracker's OnChanged method, but we've seen the objects coming through
		// this path missing TypeMeta, so ensure it is properly populated.
		controller.EnsureTypeMeta(
			tracker.OnChanged,
			gvk,
		),
	))
	i.concrete[gvr] = struct{}{}
	return nil
}
