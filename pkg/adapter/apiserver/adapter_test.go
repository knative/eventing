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

package apiserver

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubetesting "k8s.io/client-go/testing"
	adaptertest "knative.dev/eventing/pkg/adapter/v2/test"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/eventfilter/subscriptionsapi"
	rectesting "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/pkg/logging"
	pkgtesting "knative.dev/pkg/reconciler/testing"
)

const apiServerSourceNameTest = "test-apiserversource"

func TestAdapter_StartRef(t *testing.T) {
	ce := adaptertest.NewTestClient()

	config := Config{
		Namespaces: []string{"default"},
		Resources: []ResourceWatch{{
			GVR: schema.GroupVersionResource{
				Version:  "v1",
				Resource: "pods",
			},
		}},
		EventMode: "Resource",
	}
	ctx, _ := pkgtesting.SetupFakeContext(t)

	a := &apiServerAdapter{
		ce:     ce,
		logger: logging.FromContext(ctx),
		config: config,

		discover: makeDiscoveryClient(),
		k8s:      makeDynamicClient(simplePod("foo", "default")),
		source:   "unit-test",
		name:     "unittest",
	}

	err := errors.New("test never ran")

	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		err = a.Start(ctx)
		close(done)
	}()

	// Wait for the reflector to be fully initialized.
	// Ideally we want to check LastSyncResourceVersion is not empty but we
	// don't have access to it.
	time.Sleep(1 * time.Second)

	cancel()
	<-done

	if err != nil {
		t.Error("Did not expect an error, but got:", err)
	}
}

func TestAdapter_StartResource(t *testing.T) {
	ce := adaptertest.NewTestClient()

	config := Config{
		Namespaces: []string{"default"},
		Resources: []ResourceWatch{{
			GVR: schema.GroupVersionResource{
				Version:  "v1",
				Resource: "pods",
			},
		}},
		EventMode: "Resource",
	}
	ctx, _ := pkgtesting.SetupFakeContext(t)

	a := &apiServerAdapter{
		ce:     ce,
		logger: logging.FromContext(ctx),
		config: config,

		discover: makeDiscoveryClient(),
		k8s:      makeDynamicClient(simplePod("foo", "default")),
		source:   "unit-test",
		name:     "unittest",
	}

	err := errors.New("test never ran")
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		err = a.Start(ctx)
		close(done)
	}()

	// Wait for the reflector to be fully initialized.
	// Ideally we want to check LastSyncResourceVersion is not empty but we
	// don't have access to it.
	time.Sleep(1 * time.Second)

	cancel()
	<-done

	if err != nil {
		t.Error("Did not expect an error, but got:", err)
	}
}

func TestAdapter_StartNonNamespacedResource(t *testing.T) {
	ce := adaptertest.NewTestClient()

	config := Config{
		Namespaces: []string{"default"},
		Resources: []ResourceWatch{{
			GVR: schema.GroupVersionResource{
				Version:  "v1",
				Resource: "namespaces",
			},
		}},
		EventMode: "Resource",
	}
	ctx, _ := pkgtesting.SetupFakeContext(t)

	a := &apiServerAdapter{
		ce:     ce,
		logger: logging.FromContext(ctx),
		config: config,

		discover: makeDiscoveryClient(),
		k8s:      makeDynamicClient(simpleNamespace("foo")),
		source:   "unit-test",
		name:     "unittest",
	}

	err := errors.New("test never ran")
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		err = a.Start(ctx)
		done <- struct{}{}
	}()

	// Wait for the reflector to be fully initialized.
	// Ideally we want to check LastSyncResourceVersion is not empty but we
	// don't have access to it.
	time.Sleep(1 * time.Second)

	cancel()
	<-done

	if err != nil {
		t.Error("Did not expect an error, but got: ", err)
	}
}

// Common methods:

// GetDynamicClient returns the mockDynamicClient to use for this test case.
func makeDynamicClient(objects ...runtime.Object) dynamic.Interface {
	sc := runtime.NewScheme()
	_ = corev1.AddToScheme(sc)
	dynamicMocks := rectesting.DynamicMocks{} // TODO: maybe we need to customize this.
	realInterface := dynamicfake.NewSimpleDynamicClient(sc, objects...)
	return rectesting.NewMockDynamicInterface(realInterface, dynamicMocks)
}

func makeDiscoveryClient() discovery.DiscoveryInterface {
	return &discoveryfake.FakeDiscovery{
		Fake: &kubetesting.Fake{
			Resources: []*metav1.APIResourceList{
				{
					GroupVersion: "v1",
					APIResources: []metav1.APIResource{
						// All resources used at tests need to be listed here
						{
							Name:       "pods",
							Namespaced: true,
							Group:      "",
							Version:    "v1",
							Kind:       "Pod",
						},
						{
							Name:       "namespaces",
							Namespaced: false,
							Group:      "",
							Version:    "v1",
							Kind:       "Namespace",
						},
					},
				},
			},
		},
	}
}

func simplePod(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
		},
	}
}

func simpleNamespace(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": name,
			},
		},
	}
}

func simpleOwnedPod(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      "owned",
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "apps/v1",
						"blockOwnerDeletion": true,
						"controller":         true,
						"kind":               "ReplicaSet",
						"name":               name,
						"uid":                "0c119059-7113-11e9-a6c5-42010a8a00ed",
					},
				},
			},
		},
	}
}

func validateSent(t *testing.T, ce *adaptertest.TestCloudEventsClient, want string) {
	if got := len(ce.Sent()); got != 1 {
		t.Error("Expected 1 event to be sent, got:", got)
	}

	if got := ce.Sent()[0].Type(); got != want {
		t.Errorf("Expected %q event to be sent, got %q", want, got)
	}
}

func validateNotSent(t *testing.T, ce *adaptertest.TestCloudEventsClient, want string) {
	if got := len(ce.Sent()); got != 0 {
		t.Error("Expected 0 events to be sent, got:", got)
	}
}

func makeResourceAndTestingClient() (*resourceDelegate, *adaptertest.TestCloudEventsClient) {
	ce := adaptertest.NewTestClient()
	logger := zap.NewExample().Sugar()

	return &resourceDelegate{
		ce:                  ce,
		source:              "unit-test",
		apiServerSourceName: apiServerSourceNameTest,
		logger:              logger,
		filter:              subscriptionsapi.NewAllFilter(subscriptionsapi.MaterializeFiltersList(logger.Desugar(), []eventingv1.SubscriptionsAPIFilter{})...),
	}, ce
}

func makeRefAndTestingClient() (*resourceDelegate, *adaptertest.TestCloudEventsClient) {
	ce := adaptertest.NewTestClient()
	logger := zap.NewExample().Sugar()

	return &resourceDelegate{
		ce:                  ce,
		source:              "unit-test",
		apiServerSourceName: apiServerSourceNameTest,
		logger:              zap.NewExample().Sugar(),
		ref:                 true,
		filter:              subscriptionsapi.NewAllFilter(subscriptionsapi.MaterializeFiltersList(logger.Desugar(), []eventingv1.SubscriptionsAPIFilter{})...),
	}, ce
}

func TestAdapter_FailFast(t *testing.T) {
	ce := adaptertest.NewTestClient()

	tests := []struct {
		name               string
		failFast           bool
		expectedLogMessage string
	}{
		{
			name:               "fail fast permissions enabled",
			failFast:           true,
			expectedLogMessage: "Starting in fail-fast mode. Any single watch failure will stop the adapter.",
		},
		{
			name:     "fail fast disabled",
			failFast: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Namespaces: []string{"default"},
				Resources: []ResourceWatch{{
					GVR: schema.GroupVersionResource{
						Version:  "v1",
						Resource: "pods",
					},
				}},
				EventMode: "Resource",
				FailFast:  tt.failFast,
			}

			ctx, _ := pkgtesting.SetupFakeContext(t)
			logger := logging.FromContext(ctx).Named("adapter-test")

			a := &apiServerAdapter{
				ce:     ce,
				logger: logger,
				config: config,

				discover: makeDiscoveryClient(),
				k8s:      makeDynamicClient(simplePod("foo", "default")),
				source:   "unit-test",
				name:     "unittest",
			}

			ctx, cancel := context.WithCancel(ctx)
			done := make(chan struct{})
			go func() {
				defer close(done)
				err := a.Start(ctx)
				if err != nil {
					t.Logf("Start returned error: %v", err)
				}
			}()

			time.Sleep(1 * time.Second)

			cancel()
			<-done
		})
	}
}

func TestAdapter_DisableCache(t *testing.T) {
	ce := adaptertest.NewTestClient()

	config := Config{
		Namespaces: []string{"default"},
		Resources: []ResourceWatch{{
			GVR: schema.GroupVersionResource{
				Version:  "v1",
				Resource: "pods",
			},
		}},
		EventMode:    "Resource",
		DisableCache: true,
	}

	ctx, _ := pkgtesting.SetupFakeContext(t)

	a := &apiServerAdapter{
		ce:     ce,
		logger: logging.FromContext(ctx),
		config: config,

		discover: makeDiscoveryClient(),
		k8s:      makeDynamicClient(simplePod("foo", "default")),
		source:   "unit-test",
		name:     "unittest",
	}

	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := a.Start(ctx)
		if err != nil {
			t.Logf("Start returned error: %v", err)
		}
	}()

	cancel()
	<-done
}

// TestAdapter_DisableCacheLightweightList verifies that DisableCache=true uses a
// lightweight LIST (Limit=1) to obtain the current resourceVersion before watching,
// rather than skipping LIST entirely (which would cause rv="" and replay all existing objects).
func TestAdapter_DisableCacheLightweightList(t *testing.T) {
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	tracker := &listOptionsTracker{}

	dynClient := makeDynamicClient(simplePod("existing-pod", "default"))
	trackedClient := &listOptsTrackingClient{Interface: dynClient, gvr: gvr, tracker: tracker}

	config := Config{
		Namespaces:   []string{"default"},
		Resources:    []ResourceWatch{{GVR: gvr}},
		EventMode:    "Resource",
		DisableCache: true,
	}

	ctx, _ := pkgtesting.SetupFakeContext(t)
	ctx, cancel := context.WithCancel(ctx)

	a := &apiServerAdapter{
		ce:       adaptertest.NewTestClient(),
		logger:   logging.FromContext(ctx),
		config:   config,
		discover: makeDiscoveryClient(),
		k8s:      trackedClient,
		source:   "unit-test",
		name:     "unittest",
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = a.Start(ctx)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(tracker.get()) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done

	opts := tracker.get()
	if len(opts) == 0 {
		t.Fatal("expected at least one LIST call to obtain resourceVersion, got none")
	}
	for _, o := range opts {
		if o.Limit != 1 {
			t.Errorf("expected LIST with Limit=1 (lightweight), got Limit=%d", o.Limit)
		}
	}
}

// TestAdapter_DisableCacheEventDelivery verifies that watch events are converted
// to CloudEvents and delivered to the sink when DisableCache=true.
func TestAdapter_DisableCacheEventDelivery(t *testing.T) {
	testCE := adaptertest.NewTestClient()
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "pods"}

	fakeWatcher := watch.NewRaceFreeFake()
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme, simplePod("existing-pod", "default"))
	dynClient.PrependReactor("list", "pods", func(_ kubetesting.Action) (bool, runtime.Object, error) {
		list := &unstructured.UnstructuredList{}
		list.SetResourceVersion("100")
		return true, list, nil
	})
	dynClient.PrependWatchReactor("pods", func(_ kubetesting.Action) (bool, watch.Interface, error) {
		return true, fakeWatcher, nil
	})

	config := Config{
		Namespaces:   []string{"default"},
		Resources:    []ResourceWatch{{GVR: gvr}},
		EventMode:    "Resource",
		DisableCache: true,
	}

	ctx, _ := pkgtesting.SetupFakeContext(t)
	ctx, cancel := context.WithCancel(ctx)

	a := &apiServerAdapter{
		ce:        testCE,
		logger:    logging.FromContext(ctx),
		config:    config,
		discover:  makeDiscoveryClient(),
		k8s:       dynClient,
		source:    "unit-test",
		name:      "unittest",
		namespace: "default",
	}

	// Pre-buffer the event before starting the adapter. The fake watcher has a
	// 100-item channel; the adapter will drain it once the Watch() call is made.
	newPod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":            "new-pod",
				"namespace":       "default",
				"resourceVersion": "12345",
			},
		},
	}
	fakeWatcher.Add(newPod)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = a.Start(ctx)
	}()

	// Poll until at least one CloudEvent is sent (or timeout).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(testCE.Sent()) == 0 {
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	<-done

	if len(testCE.Sent()) == 0 {
		t.Error("expected at least one CloudEvent to be sent, but none were received")
	}
}

// listOptionsTracker records ListOptions from List calls for later assertion.
type listOptionsTracker struct {
	mu      sync.Mutex
	options []metav1.ListOptions
}

func (t *listOptionsTracker) record(opts metav1.ListOptions) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.options = append(t.options, opts)
}

func (t *listOptionsTracker) get() []metav1.ListOptions {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]metav1.ListOptions, len(t.options))
	copy(result, t.options)
	return result
}

// listOptsTrackingClient wraps dynamic.Interface to record List calls for a specific GVR.
type listOptsTrackingClient struct {
	dynamic.Interface
	gvr     schema.GroupVersionResource
	tracker *listOptionsTracker
}

func (c *listOptsTrackingClient) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &listOptsTrackingResourceClient{
		NamespaceableResourceInterface: c.Interface.Resource(gvr),
		gvr:                            gvr,
		targetGVR:                      c.gvr,
		tracker:                        c.tracker,
	}
}

type listOptsTrackingResourceClient struct {
	dynamic.NamespaceableResourceInterface
	gvr       schema.GroupVersionResource
	targetGVR schema.GroupVersionResource
	tracker   *listOptionsTracker
}

func (r *listOptsTrackingResourceClient) Namespace(ns string) dynamic.ResourceInterface {
	return &listOptsTrackingNamespacedClient{
		ResourceInterface: r.NamespaceableResourceInterface.Namespace(ns),
		gvr:               r.gvr,
		targetGVR:         r.targetGVR,
		tracker:           r.tracker,
	}
}

type listOptsTrackingNamespacedClient struct {
	dynamic.ResourceInterface
	gvr       schema.GroupVersionResource
	targetGVR schema.GroupVersionResource
	tracker   *listOptionsTracker
}

func (r *listOptsTrackingNamespacedClient) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	if r.gvr == r.targetGVR {
		r.tracker.record(opts)
	}
	return r.ResourceInterface.List(ctx, opts)
}
