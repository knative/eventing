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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	gotesting "testing"
	"time"

	"github.com/cloudevents/sdk-go/pkg/cloudevents/datacodec/json"
	"github.com/google/go-cmp/cmp"
	"github.com/knative/eventing/pkg/kncloudevents"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	logtesting "github.com/knative/pkg/logging/testing"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakedynamicclientset "k8s.io/client-go/dynamic/fake"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
)

const (
	sourceName = "test-apiserver-adapter"
	sourceUID  = "1234-5678-90"
	testNS     = "testnamespace"
)

type testCase struct {
	// Name is a descriptive name for this test suitable as a first argument to t.Run()
	Name string

	// InitialState is the list of objects that already exists when reconciliation
	// starts.
	InitialState []runtime.Object

	// Key is the parameter to reconciliation.
	// This has the form "namespace/name".
	Key string

	// Where to send events
	sink func(http.ResponseWriter, *http.Request)

	// Expected event data
	data interface{}
}

func TestReconcile(t *gotesting.T) {
	table := []testCase{
		{
			Name: "Receive Pod creation event",
			InitialState: []runtime.Object{
				getPod(),
			},
			Key: testNS + "/" + sourceName,

			sink: sinkAccepted,
			data: decode(t, encode(t, getPodRef())),
		},
	}

	for _, tc := range table {
		t.Run(tc.Name, func(t *gotesting.T) {
			// Create fake sink server
			h := &fakeHandler{
				handler: tc.sink,
			}

			sinkServer := httptest.NewServer(h)
			defer sinkServer.Close()

			// Bind cloud event client
			ceClient, err := kncloudevents.NewDefaultClient(sinkServer.URL)
			if err != nil {
				t.Errorf("cannot create cloud event client: %v", zap.Error(err))
			}

			// Create fake dynamic client
			dynamicScheme := runtime.NewScheme()
			client := fakedynamicclientset.NewSimpleDynamicClient(dynamicScheme, tc.InitialState...)

			stopCh := make(chan struct{})
			defer close(stopCh)

			tif := &duck.TypedInformerFactory{
				Client:       client,
				Type:         &duckv1alpha1.AddressableType{},
				ResyncPeriod: 1 * time.Second,
				StopChannel:  stopCh,
			}

			_, lister, err := tif.Get(schema.GroupVersionResource{Group: "", Resource: "pods", Version: "v1"})
			if err != nil {
				t.Fatalf("Get() = %v", err)
			}

			opt := reconciler.Options{
				KubeClientSet: fakekubeclientset.NewSimpleClientset(),
				Logger:        logtesting.TestLogger(t),
			}

			r := &Reconciler{
				Base:         reconciler.NewBase(opt, controllerAgentName),
				eventsClient: ceClient,
				lister:       lister,
			}
			ctx := context.Background()

			err = r.Reconcile(ctx, tc.Key)
			if err != nil {
				t.Errorf("Expected no error")
			}

			if diff := cmp.Diff(tc.data, decode(t, h.body)); diff != "" {
				t.Errorf("incorrect event (-want, +got): %v", diff)
			}
		})
	}

}

func getPod() runtime.Object {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      sourceName,
				"selfLink":  "/apis/v1/namespaces/" + testNS + "/pod/" + sourceName,
			},
		},
	}
}

func getPodRef() corev1.ObjectReference {
	return corev1.ObjectReference{
		APIVersion: "v1",
		Kind:       "Pod",
		Name:       sourceName,
		Namespace:  testNS,
	}
}

type fakeHandler struct {
	body   []byte
	header http.Header

	handler func(http.ResponseWriter, *http.Request)
}

func (h *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.header = r.Header
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "can not read body", http.StatusBadRequest)
		return
	}
	h.body = body
	defer r.Body.Close()
	h.handler(w, r)
}

func sinkAccepted(writer http.ResponseWriter, req *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

func encode(t *gotesting.T, data interface{}) string {
	b, err := json.Encode(data)
	if err != nil {
		t.Fatalf("failed to encode data: %v", err)
	}
	return string(b)
}

func decode(t *gotesting.T, data interface{}) interface{} {
	var out interface{}
	err := json.Decode(data, &out)
	if err != nil {
		t.Fatalf("failed to decode data: %v", err)
	}
	return out
}
