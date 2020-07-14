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
package cache

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
)

type KResourceWithSpec struct {
	duckv1.KResource
	Spec interface{}
}

const (
	cmNs   = "test-ns"
	cmName = "test-name"
)

var (
	// Check that Channel can return its spec untyped.
	_ apis.HasSpec = (*KResourceWithSpec)(nil)
)

func (k *KResourceWithSpec) GetUntypedSpec() interface{} {
	return k.Spec
}

func TestPersistedStore(t *testing.T) {
	const (
		cmNs   = "test-cm-ns"
		cmName = "test-cm-name"
	)
	cs := fake.NewSimpleClientset()
	created := make(chan runtime.Object)
	updated := make(chan runtime.Object)
	done := make(chan bool)
	cs.PrependReactor("create", "configmaps",
		func(action ktesting.Action) (bool, runtime.Object, error) {
			created <- action.(ktesting.CreateAction).GetObject()
			return false, nil, nil
		},
	)
	cs.PrependReactor("update", "configmaps",
		func(action ktesting.Action) (bool, runtime.Object, error) {
			updated <- action.(ktesting.UpdateAction).GetObject()
			return false, nil, nil
		},
	)

	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	informer := newKResourceInformer(store)

	logger := logtesting.TestLogger(t)

	pstore := NewPersistedStore("my-component", cs, cmNs, cmName, informer, func(obj interface{}) interface{} {
		return obj.(apis.HasSpec).GetUntypedSpec()
	})
	ctx := logging.WithLogger(context.Background(), logger)
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		pstore.Run(ctx)
		done <- true
	}()

	kr := newKResource("test-ns", "test-name")
	informer.Add(kr)

	select {
	case obj := <-created:
		// We expect the configmap to be created.
		cm := obj.(*corev1.ConfigMap)
		if value, ok := cm.Labels[ComponentLabelKey]; !ok || value != "my-component" {
			t.Fatalf("Missing %s label. Got %v", ComponentLabelKey, cm)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for configmap creation.")
	}

	select {
	case obj := <-updated:
		cm := obj.(*corev1.ConfigMap)
		if value, ok := cm.Data["resources.json"]; !ok || value != `{"test-ns/test-name":"aspec"}` {
			t.Fatalf("Unexpected ConfigMap. Got %v", cm)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for configmap update.")
	}

	informer.Delete(kr)

	select {
	case obj := <-updated:
		cm := obj.(*corev1.ConfigMap)
		if spec, ok := cm.Data["resources.json"]; !ok || spec != `{}` {
			t.Fatalf("Unexpected ConfigMap. Got %v", cm)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for configmap update.")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for persisted store to stop.")
	}

}

func TestPersistedStoreUnStarted(t *testing.T) {
	cs := fake.NewSimpleClientset()
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	informer := newKResourceInformer(store)
	pstore := NewPersistedStore("my-component", cs, cmNs, cmName, informer, nil).(*persistedStore)

	done := make(chan bool)
	go func() {
		pstore.sync()
		pstore.sync()
		pstore.sync()
		done <- true
	}()
	select {
	case <-done:
		// All good, none blocking
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out while waiting for multiple sync triggers to terminate.")
	}
}

func TestPersistedStoreInterrupted(t *testing.T) {
	const (
		cmNs   = "test-cm-ns"
		cmName = "test-cm-name"
	)
	cs := fake.NewSimpleClientset()
	created := make(chan runtime.Object)
	updated := make(chan runtime.Object)
	cs.PrependReactor("create", "configmaps",
		func(action ktesting.Action) (bool, runtime.Object, error) {
			time.Sleep(1 * time.Second)
			created <- action.(ktesting.CreateAction).GetObject()
			return false, nil, nil
		},
	)
	cs.PrependReactor("update", "configmaps",
		func(action ktesting.Action) (bool, runtime.Object, error) {
			updated <- action.(ktesting.UpdateAction).GetObject()
			return false, nil, nil
		},
	)

	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	informer := newKResourceInformer(store)
	logger := logtesting.TestLogger(t)

	pstore := NewPersistedStore("my-component", cs, cmNs, cmName, informer, func(obj interface{}) interface{} {
		return obj.(apis.HasSpec).GetUntypedSpec()
	})

	ctx := logging.WithLogger(context.Background(), logger)
	go func() {
		pstore.Run(ctx)
	}()

	kr := newKResource("test-ns", "test-name")
	informer.Add(kr)

	time.Sleep(500 * time.Millisecond)

	kr2 := newKResource("test-ns", "test-name-2")
	informer.Add(kr2) // interrupt

	select {
	case obj := <-created:
		// We expect the configmap to be created.
		cm := obj.(*corev1.ConfigMap)
		if value, ok := cm.Labels[ComponentLabelKey]; !ok || value != "my-component" {
			t.Fatalf("Missing %s label. Got %v", ComponentLabelKey, cm)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for configmap creation.")
	}

	select {
	case obj := <-updated:
		cm := obj.(*corev1.ConfigMap)
		if value, ok := cm.Data["resources.json"]; !ok || value != `{"test-ns/test-name":"aspec","test-ns/test-name-2":"aspec"}` {
			t.Fatalf("Unexpected ConfigMap. Got %v", cm)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for configmap update.")
	}
}

func newKResource(ns, name string) *KResourceWithSpec {
	return &KResourceWithSpec{
		KResource: duckv1.KResource{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{
					{
						Type:   apis.ConditionReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		Spec: "aspec",
	}
}

type kresourceInformer struct {
	store   cache.Store
	handler cache.ResourceEventHandler
}

func newKResourceInformer(store cache.Store) *kresourceInformer {
	return &kresourceInformer{
		store: store,
	}
}

func (k *kresourceInformer) Add(obj interface{}) {
	k.store.Add(obj)
	k.handler.OnAdd(obj)
}

func (k *kresourceInformer) Delete(obj interface{}) {
	k.store.Delete(obj)
	k.handler.OnDelete(obj)
}

func (k *kresourceInformer) AddEventHandler(handler cache.ResourceEventHandler) {
	k.handler = handler
}

func (k *kresourceInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) {
}

func (k *kresourceInformer) GetStore() cache.Store {
	return k.store
}

func (k *kresourceInformer) GetController() cache.Controller {
	return nil
}

func (k *kresourceInformer) Run(stopCh <-chan struct{}) {
}

func (k *kresourceInformer) HasSynced() bool {
	return true
}

func (k *kresourceInformer) LastSyncResourceVersion() string {
	return ""
}
