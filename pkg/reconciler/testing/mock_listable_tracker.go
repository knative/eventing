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

package testing

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/duck"
)

// MockListableTracker is a mock ListableTracker.
type MockListableTracker struct {
	err     error
	listers map[schema.GroupVersionResource]cache.GenericLister
}

var _ duck.ListableTracker = (*MockListableTracker)(nil)

// TrackInNamespace implements the ListableTracker interface.
func (t *MockListableTracker) TrackInNamespace(metav1.Object) func(corev1.ObjectReference) error {
	return func(corev1.ObjectReference) error {
		return t.err
	}
}

// ListerFor implements the ListableTracker interface.
func (t *MockListableTracker) ListerFor(ref corev1.ObjectReference) (cache.GenericLister, error) {
	gvk := ref.GroupVersionKind()
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	if lister, ok := t.listers[gvr]; ok {
		return lister, nil
	}
	return nil, fmt.Errorf("lister not mocked for GVR %s", gvr.String())
}

// InformerFor implements the ListableTracker interface.
func (t *MockListableTracker) InformerFor(_ corev1.ObjectReference) (cache.SharedIndexInformer, error) {
	return nil, nil
}
