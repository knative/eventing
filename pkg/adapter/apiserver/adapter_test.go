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
	"testing"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubetesting "k8s.io/client-go/testing"
	adaptertest "knative.dev/eventing/pkg/adapter/v2/test"
	rectesting "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/pkg/logging"
	pkgtesting "knative.dev/pkg/reconciler/testing"
)

const apiServerSourceNameTest = "test-apiserversource"

func TestAdapter_StartRef(t *testing.T) {
	ce := adaptertest.NewTestClient()

	config := Config{
		Namespace: "default",
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
		Namespace: "default",
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
		Namespace: "default",
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
	return &resourceDelegate{
		ce:                  ce,
		source:              "unit-test",
		apiServerSourceName: apiServerSourceNameTest,
		logger:              zap.NewExample().Sugar(),
	}, ce
}

func makeRefAndTestingClient() (*resourceDelegate, *adaptertest.TestCloudEventsClient) {
	ce := adaptertest.NewTestClient()
	return &resourceDelegate{
		ce:                  ce,
		source:              "unit-test",
		apiServerSourceName: apiServerSourceNameTest,
		logger:              zap.NewExample().Sugar(),
		ref:                 true,
	}, ce
}
