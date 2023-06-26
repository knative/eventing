/* Copyright 2023 The Knative Authors

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

package resolver

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	pkgapisduck "knative.dev/pkg/apis/duck"
	v1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/kresource"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/tracker"
)

type KReferenceResolver struct {
	tracker       tracker.Interface
	listerFactory func(schema.GroupVersionResource) (cache.GenericLister, error)
}

func NewKReferenceResolverFromTracker(ctx context.Context, t tracker.Interface) *KReferenceResolver {
	ret := &KReferenceResolver{
		tracker: t,
	}

	informerFactory := &pkgapisduck.CachedInformerFactory{
		Delegate: &pkgapisduck.EnqueueInformerFactory{
			Delegate:     kresource.Get(ctx),
			EventHandler: controller.HandleAll(ret.tracker.OnChanged),
		},
	}

	ret.listerFactory = func(gvr schema.GroupVersionResource) (cache.GenericLister, error) {
		_, l, err := informerFactory.Get(ctx, gvr)
		return l, err
	}

	return ret
}

func (r *KReferenceResolver) Resolve(ctx context.Context, ref *v1.KReference, parent interface{}) (*v1.KReference, error) {

	if ref == nil {
		return nil, apierrs.NewBadRequest("ref is nil")
	}

	or := &corev1.ObjectReference{
		Kind:       ref.Kind,
		Namespace:  ref.Namespace,
		Name:       ref.Name,
		APIVersion: ref.APIVersion,
	}

	gvr, _ := meta.UnsafeGuessKindToResource(or.GroupVersionKind())

	if err := r.tracker.TrackReference(tracker.Reference{
		APIVersion: ref.APIVersion,
		Kind:       ref.Kind,
		Namespace:  ref.Namespace,
		Name:       ref.Name,
	}, parent); err != nil {
		return nil, fmt.Errorf("failed to track reference %s %s/%s: %w", gvr.String(), ref.Namespace, ref.Name, err)
	}

	lister, err := r.listerFactory(gvr)
	if err != nil {
		return nil, fmt.Errorf("failed to get lister for %s: %w", gvr.String(), err)
	}

	_, err = lister.ByNamespace(ref.Namespace).Get(ref.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get object %s/%s: %w", ref.Namespace, ref.Name, err)
	}

	return ref, nil
}
