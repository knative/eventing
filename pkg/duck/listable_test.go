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
	"errors"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis/duck"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
)

func init() {
	// Add types to scheme
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)
}

const (
	ns = "test-ns"
)

var (
	errTest = errors.New("test error")
)

type fakeInformerFactory struct {
	gvr map[schema.GroupVersionResource]int
	err error
}

var _ duck.InformerFactory = (*fakeInformerFactory)(nil)

func (fif *fakeInformerFactory) Get(gvr schema.GroupVersionResource) (cache.SharedIndexInformer, cache.GenericLister, error) {
	if fif.err != nil {
		return nil, nil, fif.err
	}
	if _, present := fif.gvr[gvr]; !present {
		fif.gvr[gvr] = 0
	}
	fif.gvr[gvr]++
	return &fakeInformer{}, nil, nil
}

func newFakeInformerFactory() *fakeInformerFactory {
	return &fakeInformerFactory{
		gvr: map[schema.GroupVersionResource]int{},
	}
}

func TestResourceTracker(t *testing.T) {
	testCases := map[string]struct {
		informerFactoryError error
		repeatedTracks       int
		expectedError        error
	}{
		"informerFactory error": {
			informerFactoryError: errTest,
			expectedError:        errTest,
		},
		"Only one informer created per GVR": {
			repeatedTracks: 1,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			fif := newFakeInformerFactory()
			if tc.informerFactoryError != nil {
				fif.err = tc.informerFactoryError
			}
			ctx, _ := fakedynamicclient.With(context.Background(), scheme.Scheme)
			tr := NewListableTracker(ctx, &eventingduckv1alpha1.Resource{}, func(types.NamespacedName) {}, time.Minute)
			rt, _ := tr.(*listableTracker)
			rt.informerFactory = fif
			track := rt.TrackInNamespace(&corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns,
					Name:      "svc",
				},
			})
			for i := 0; i <= tc.repeatedTracks; i++ {
				ref := corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       fmt.Sprintf("ref-%d", i),
				}
				err := track(ref)
				if tc.expectedError != nil {
					if err != tc.expectedError {
						t.Fatalf("Incorrect error from returned track function. Expected '%v'. Actual '%v'", tc.expectedError, err)
					}
					return
				}
			}
			gvr := schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			}
			if fif.gvr[gvr] != 1 {
				t.Fatalf("Unexpected number of calls to the Informer factory. Expected 1. Actual %d.", fif.gvr[gvr])
			}
		})
	}
}

type fakeInformer struct {
	eventHandlerAdded bool
}

func (fi *fakeInformer) AddEventHandler(cache.ResourceEventHandler) {
	fi.eventHandlerAdded = true
}

func (fi *fakeInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) {
	panic("not used in the test")
}

func (fi *fakeInformer) GetStore() cache.Store {
	panic("not used in the test")
}

func (fi *fakeInformer) GetController() cache.Controller {
	panic("not used in the test")
}

func (fi *fakeInformer) Run(stopCh <-chan struct{}) {
	panic("not used in the test")
}

func (fi *fakeInformer) HasSynced() bool {
	panic("not used in the test")
}

func (fi *fakeInformer) LastSyncResourceVersion() string {
	panic("not used in the test")
}

func (fi *fakeInformer) AddIndexers(indexers cache.Indexers) error {
	panic("not used in the test")
}

func (fi *fakeInformer) GetIndexer() cache.Indexer {
	panic("not used in the test")
}

var _ cache.SharedIndexInformer = (*fakeInformer)(nil)
