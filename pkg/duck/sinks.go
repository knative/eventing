/*
Copyright 2018 The Knative Authors

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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"

	"knative.dev/eventing/pkg/reconciler/names"
	pkgapisduck "knative.dev/pkg/apis/duck"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/client/injection/ducks/duck/v1alpha1/addressable"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/tracker"
)

// SinkReconciler is a helper for Sources. It triggers
// reconciliation on creation, updates to or deletion of the source's sink.
type SinkReconciler struct {
	tracker             tracker.Interface
	sinkInformerFactory pkgapisduck.InformerFactory
}

// NewSinkReconciler creates and initializes a new SinkReconciler
func NewSinkReconciler(ctx context.Context, callback func(types.NamespacedName)) *SinkReconciler {
	ret := &SinkReconciler{}

	ret.tracker = tracker.New(callback, controller.GetTrackerLease(ctx))
	ret.sinkInformerFactory = &pkgapisduck.CachedInformerFactory{
		Delegate: &pkgapisduck.EnqueueInformerFactory{
			Delegate:     addressable.Get(ctx),
			EventHandler: controller.HandleAll(ret.tracker.OnChanged),
		},
	}

	return ret
}

// GetSinkURI registers the given object reference with the tracker and if possible,
// retrieves the sink URI
func (r *SinkReconciler) GetSinkURI(sinkObjRef *corev1.ObjectReference, source interface{}, sourceDesc string) (string, error) {
	if sinkObjRef == nil {
		return "", fmt.Errorf("sink ref is nil")
	}

	if err := r.tracker.Track(*sinkObjRef, source); err != nil {
		return "", fmt.Errorf("Error tracking sink '%+v' for source %q: %+v", sinkObjRef, sourceDesc, err)
	}

	// K8s Services are special cased. They can be called, even though they do not satisfy the
	// Callable interface.
	if sinkObjRef.APIVersion == "v1" && sinkObjRef.Kind == "Service" {
		return DomainToURL(names.ServiceHostName(sinkObjRef.Name, sinkObjRef.Namespace)), nil
	}

	gvr, _ := meta.UnsafeGuessKindToResource(sinkObjRef.GroupVersionKind())
	_, lister, err := r.sinkInformerFactory.Get(gvr)
	if err != nil {
		return "", fmt.Errorf("Error getting a lister for a sink resource '%+v': %+v", gvr, err)
	}

	sinkObj, err := lister.ByNamespace(sinkObjRef.Namespace).Get(sinkObjRef.Name)
	if err != nil {
		return "", fmt.Errorf("Error fetching sink %+v for source %q: %v", sinkObjRef, sourceDesc, err)
	}

	sink, ok := sinkObj.(*duckv1alpha1.AddressableType)
	if !ok {
		return "", fmt.Errorf("object %+v is of the wrong kind", sinkObjRef)
	}
	if sink.Status.Address == nil {
		return "", fmt.Errorf("sink %+v does not contain address", sinkObjRef)
	}
	url := sink.Status.Address.GetURL()
	if url.Host == "" {
		return "", fmt.Errorf("sink %+v contains an empty hostname", sinkObjRef)
	}
	return url.String(), nil
}
