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
	"testing"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	adaptertest "knative.dev/eventing/pkg/adapter/v2/test"
	rectesting "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/pkg/logging"
	pkgtesting "knative.dev/pkg/reconciler/testing"
)

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
		k8s:    makeDynamicClient(simplePod("foo", "default")),
		source: "unit-test",
		name:   "unittest",
	}

	err := errors.New("test never ran")
	stopCh := make(chan struct{})
	done := make(chan struct{})
	go func() {
		err = a.Start(stopCh)
		done <- struct{}{}
	}()

	// Wait for the reflector to be fully initialized.
	// Ideally we want to check LastSyncResourceVersion is not empty but we
	// don't have access to it.
	time.Sleep(1 * time.Second)

	stopCh <- struct{}{}
	<-done

	if err != nil {
		t.Errorf("did not expect an error, but got %v", err)
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
		k8s:    makeDynamicClient(simplePod("foo", "default")),
		source: "unit-test",
		name:   "unittest",
	}

	err := errors.New("test never ran")
	stopCh := make(chan struct{})
	done := make(chan struct{})
	go func() {
		err = a.Start(stopCh)
		done <- struct{}{}
	}()

	// Wait for the reflector to be fully initialized.
	// Ideally we want to check LastSyncResourceVersion is not empty but we
	// don't have access to it.
	time.Sleep(1 * time.Second)

	stopCh <- struct{}{}
	<-done

	if err != nil {
		t.Errorf("did not expect an error, but got %v", err)
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
		t.Errorf("Expected 1 event to be sent, got %d", got)
	}

	if got := ce.Sent()[0].Type(); got != want {
		t.Errorf("Expected %q event to be sent, got %q", want, got)
	}
}

func validateNotSent(t *testing.T, ce *adaptertest.TestCloudEventsClient, want string) {
	if got := len(ce.Sent()); got != 0 {
		t.Errorf("Expected 0 event to be sent, got %d", got)
	}
}

func makeResourceAndTestingClient() (*resourceDelegate, *adaptertest.TestCloudEventsClient) {
	ce := adaptertest.NewTestClient()
	return &resourceDelegate{
		ce:     ce,
		source: "unit-test",
		logger: zap.NewExample().Sugar(),
	}, ce
}

func makeRefAndTestingClient() (*resourceDelegate, *adaptertest.TestCloudEventsClient) {
	ce := adaptertest.NewTestClient()
	return &resourceDelegate{
		ce:     ce,
		source: "unit-test",
		logger: zap.NewExample().Sugar(),
		ref:    true,
	}, ce
}
