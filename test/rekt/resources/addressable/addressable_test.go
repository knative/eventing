/*
Copyright 2026 The Knative Authors

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

package addressable

import (
	"context"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/reconciler-test/pkg/environment"
)

const (
	testNamespace = "test-ns"
	testName      = "my-broker"
)

var testGVR = schema.GroupVersionResource{Group: "eventing.knative.dev", Version: "v1", Resource: "brokers"}

// fakeEnvironment satisfies environment.Environment by only implementing
// Namespace(), which is all k8s.Address needs from it.
type fakeEnvironment struct {
	environment.Environment
	ns string
}

func (f *fakeEnvironment) Namespace() string { return f.ns }

// newFakeBroker returns a context wired to a fake dynamic client that holds a
// single Broker whose status.address.url is url.
func newFakeBroker(url string) (context.Context, *dynamicfake.FakeDynamicClient) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "eventing.knative.dev/v1",
			"kind":       "Broker",
			"metadata": map[string]interface{}{
				"name":      testName,
				"namespace": testNamespace,
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}{
					"url": url,
				},
			},
		},
	}

	dc := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{testGVR: "BrokerList"}, obj)

	ctx := context.WithValue(context.Background(), dynamicclient.Key{}, dc)
	ctx = environment.ContextWith(ctx, &fakeEnvironment{ns: testNamespace})
	return ctx, dc
}

// failFirstGets makes the first n gets of the Broker fail with err.
func failFirstGets(dc *dynamicfake.FakeDynamicClient, n int, err error) {
	calls := 0
	dc.PrependReactor("get", testGVR.Resource, func(clientgotesting.Action) (bool, runtime.Object, error) {
		calls++
		if calls <= n {
			return true, nil, err
		}
		return false, nil, nil
	})
}

// TestValidateAddress_RetriesUntilValid ensures ValidateAddress keeps polling
// until the fetched address actually satisfies the validate function, rather
// than validating once as soon as any address appears. A Broker (or any
// addressable) can report an address before it satisfies a given predicate,
// e.g. reporting an http:// address before its TLS address is ready.
func TestValidateAddress_RetriesUntilValid(t *testing.T) {
	ctx, dc := newFakeBroker("http://my-broker.test-ns.svc.cluster.local")

	// Simulate the address flipping to https shortly after the requirement
	// starts polling, the way a Broker's address does once its TLS
	// certificate becomes ready after first being reported over http.
	go func() {
		time.Sleep(60 * time.Millisecond)
		u, err := dc.Resource(testGVR).Namespace(testNamespace).Get(ctx, testName, metav1.GetOptions{})
		if err != nil {
			return
		}
		_ = unstructured.SetNestedField(u.Object, "https://my-broker.test-ns.svc.cluster.local", "status", "address", "url")
		_, _ = dc.Resource(testGVR).Namespace(testNamespace).Update(ctx, u, metav1.UpdateOptions{})
	}()

	step := ValidateAddress(testGVR, testName, AssertHTTPSAddress, 20*time.Millisecond, 2*time.Second)
	step(ctx, t)

	if t.Failed() {
		t.Fatal("ValidateAddress should have retried until the address became https, not failed on the initial http address")
	}
}

func TestAddress_RetriesTransientErrors(t *testing.T) {
	tests := map[string]error{
		"not found":         apierrors.NewNotFound(testGVR.GroupResource(), testName),
		"timeout":           apierrors.NewTimeoutError("request timed out", 1),
		"server timeout":    apierrors.NewServerTimeout(testGVR.GroupResource(), "get", 1),
		"too many requests": apierrors.NewTooManyRequests("throttled", 1),
	}
	for name, transientErr := range tests {
		t.Run(name, func(t *testing.T) {
			ctx, dc := newFakeBroker("https://my-broker.test-ns.svc.cluster.local")
			failFirstGets(dc, 2, transientErr)

			addr, err := Address(ctx, testGVR, testName, 10*time.Millisecond, 2*time.Second)
			if err != nil {
				t.Fatalf("Address() should have retried past %q, got error: %v", name, err)
			}
			if addr == nil || addr.URL == nil || addr.URL.String() != "https://my-broker.test-ns.svc.cluster.local" {
				t.Fatalf("Address() = %v, want the Broker's address", addr)
			}
		})
	}
}

func TestAddress_FailsFastOnNonTransientError(t *testing.T) {
	ctx, dc := newFakeBroker("https://my-broker.test-ns.svc.cluster.local")
	failFirstGets(dc, 1, apierrors.NewForbidden(testGVR.GroupResource(), testName, nil))

	start := time.Now()
	if _, err := Address(ctx, testGVR, testName, 10*time.Millisecond, 5*time.Second); !apierrors.IsForbidden(err) {
		t.Fatalf("Address() error = %v, want a Forbidden error", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Address() took %v, want it to stop polling on a non-transient error", elapsed)
	}
}
