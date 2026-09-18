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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/reconciler-test/pkg/environment"
)

// fakeEnvironment satisfies environment.Environment by only implementing
// Namespace(), which is all k8s.Address needs from it.
type fakeEnvironment struct {
	environment.Environment
	ns string
}

func (f *fakeEnvironment) Namespace() string { return f.ns }

// TestValidateAddress_RetriesUntilValid ensures ValidateAddress keeps polling
// until the fetched address actually satisfies the validate function, rather
// than validating once as soon as any address appears. A Broker (or any
// addressable) can report an address before it satisfies a given predicate,
// e.g. reporting an http:// address before its TLS address is ready.
func TestValidateAddress_RetriesUntilValid(t *testing.T) {
	const ns = "test-ns"
	const name = "my-broker"
	gvr := schema.GroupVersionResource{Group: "eventing.knative.dev", Version: "v1", Resource: "brokers"}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "eventing.knative.dev/v1",
			"kind":       "Broker",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": ns,
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}{
					"url": "http://my-broker.test-ns.svc.cluster.local",
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	dc := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{gvr: "BrokerList"}, obj)

	ctx := context.WithValue(context.Background(), dynamicclient.Key{}, dc)
	ctx = environment.ContextWith(ctx, &fakeEnvironment{ns: ns})

	// Simulate the address flipping to https shortly after the requirement
	// starts polling, the way a Broker's address does once its TLS
	// certificate becomes ready after first being reported over http.
	go func() {
		time.Sleep(60 * time.Millisecond)
		u, err := dc.Resource(gvr).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return
		}
		_ = unstructured.SetNestedField(u.Object, "https://my-broker.test-ns.svc.cluster.local", "status", "address", "url")
		_, _ = dc.Resource(gvr).Namespace(ns).Update(ctx, u, metav1.UpdateOptions{})
	}()

	step := ValidateAddress(gvr, name, AssertHTTPSAddress, 20*time.Millisecond, 2*time.Second)
	step(ctx, t)

	if t.Failed() {
		t.Fatal("ValidateAddress should have retried until the address became https, not failed on the initial http address")
	}
}
