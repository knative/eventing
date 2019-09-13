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

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kncetesting "knative.dev/eventing/pkg/kncloudevents/testing"
	rectesting "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/pkg/source"
)

type mockReporter struct {
	eventCount int
}

func (r *mockReporter) ReportEventCount(args *source.ReportArgs, responseCode int) error {
	r.eventCount += 1
	return nil
}

func TestNewAdaptor(t *testing.T) {
	ce := kncetesting.NewTestClient()
	logger := zap.NewExample().Sugar()
	k8s := makeDynamicClient(nil)

	testCases := map[string]struct {
		opt    Options
		source string

		wantMode      string
		wantNamespace string
		wantGVRCs     []GVRC
	}{
		"empty opts": {
			opt:      Options{},
			wantMode: RefMode,
		},
		"with source": {
			source:   "test-source",
			opt:      Options{},
			wantMode: RefMode,
		},
		"with namespace": {
			source: "test-source",
			opt: Options{
				Namespace: "test-ns",
			},
			wantMode:      RefMode,
			wantNamespace: "test-ns",
		},
		"with mode resource": {
			source: "test-source",
			opt: Options{
				Mode: ResourceMode,
			},
			wantMode: ResourceMode,
		},
		"with mode ref": {
			source: "test-source",
			opt: Options{
				Mode: RefMode,
			},
			wantMode: RefMode,
		},
		"with mode trash": {
			source: "test-source",
			opt: Options{
				Mode: "trash",
			},
			wantMode: RefMode,
		},
		"with mode gvrs": {
			source: "test-source",
			opt: Options{
				GVRCs: []GVRC{{
					GVR: schema.GroupVersionResource{
						Group:    "apps",
						Version:  "v1",
						Resource: "replicasets",
					},
					Controller: true,
				}},
			},
			wantMode: RefMode,
			wantGVRCs: []GVRC{{
				GVR: schema.GroupVersionResource{
					Group:    "apps",
					Version:  "v1",
					Resource: "replicasets",
				},
				Controller: true,
			}},
		},
		"with label selector": {
			source: "test-source",
			opt: Options{
				GVRCs: []GVRC{{
					GVR: schema.GroupVersionResource{
						Group:    "apps",
						Version:  "v1",
						Resource: "replicasets",
					},
					Controller:    true,
					LabelSelector: "environment=production,tier!=frontend",
				}},
			},
			wantMode: RefMode,
			wantGVRCs: []GVRC{{
				GVR: schema.GroupVersionResource{
					Group:    "apps",
					Version:  "v1",
					Resource: "replicasets",
				},
				Controller:    true,
				LabelSelector: "environment=production,tier!=frontend",
			}},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			r := &mockReporter{}
			a := NewAdaptor(tc.source, k8s, ce, logger, tc.opt, r, "test-source")

			got, ok := a.(*adapter)
			if !ok {
				t.Errorf("expected NewAdapter to return a *adapter, but did not")
			}
			if diff := cmp.Diff(tc.source, got.source); diff != "" {
				t.Errorf("unexpected source diff (-want, +got) = %v", diff)
			}
			if diff := cmp.Diff(tc.wantMode, got.mode); diff != "" {
				t.Errorf("unexpected mode diff (-want, +got) = %v", diff)
			}
			if diff := cmp.Diff(tc.wantNamespace, got.namespace); diff != "" {
				t.Errorf("unexpected namespace diff (-want, +got) = %v", diff)
			}
			if diff := cmp.Diff(tc.wantGVRCs, got.gvrcs); diff != "" {
				t.Errorf("unexpected namespace diff (-want, +got) = %v", diff)
			}
		})
	}
}

func TestAdapter_StartRef(t *testing.T) {
	ce := kncetesting.NewTestClient()
	logger := zap.NewExample().Sugar()
	k8s := makeDynamicClient(nil)
	source := "test-source"
	opt := Options{
		Mode:      RefMode,
		Namespace: "default",
		GVRCs: []GVRC{{
			GVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
		}},
	}
	r := &mockReporter{}
	a := NewAdaptor(source, k8s, ce, logger, opt, r, "test-source")

	err := errors.New("test never ran")
	stopCh := make(chan struct{})
	done := make(chan struct{})
	go func() {
		err = a.Start(stopCh)
		done <- struct{}{}
	}()

	stopCh <- struct{}{}
	<-done

	if err != nil {
		t.Errorf("did not expect an error, but got %v", err)
	}
}

func TestAdapter_StartResource(t *testing.T) {
	ce := kncetesting.NewTestClient()
	logger := zap.NewExample().Sugar()
	k8s := makeDynamicClient(nil)
	source := "test-source"
	opt := Options{
		Mode:      ResourceMode,
		Namespace: "default",
		GVRCs: []GVRC{{
			GVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
		}},
	}
	r := &mockReporter{}
	a := NewAdaptor(source, k8s, ce, logger, opt, r, "test-source")

	err := errors.New("test never ran")
	stopCh := make(chan struct{})
	done := make(chan struct{})
	go func() {
		err = a.Start(stopCh)
		done <- struct{}{}
	}()

	stopCh <- struct{}{}
	<-done

	if err != nil {
		t.Errorf("did not expect an error, but got %v", err)
	}
}

// Common methods:

// GetDynamicClient returns the mockDynamicClient to use for this test case.
func makeDynamicClient(objects []runtime.Object) dynamic.Interface {
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

func validateSent(t *testing.T, ce *kncetesting.TestCloudEventsClient, want string) {
	if got := len(ce.Sent); got != 1 {
		t.Errorf("Expected 1 event to be sent, got %d", got)
	}

	if got := ce.Sent[0].Type(); got != want {
		t.Errorf("Expected %q event to be sent, got %q", want, got)
	}
}

func validateNotSent(t *testing.T, ce *kncetesting.TestCloudEventsClient, want string) {
	if got := len(ce.Sent); got != 0 {
		t.Errorf("Expected 0 event to be sent, got %d", got)
	}
}

func makeResourceAndTestingClient() (*resource, *kncetesting.TestCloudEventsClient) {
	ce := kncetesting.NewTestClient()
	source := "unit-test"
	logger := zap.NewExample().Sugar()
	r := &mockReporter{}
	return &resource{
		ce:       ce,
		source:   source,
		logger:   logger,
		reporter: r,
	}, ce
}

func makeRefAndTestingClient() (*ref, *kncetesting.TestCloudEventsClient) {
	ce := kncetesting.NewTestClient()
	source := "unit-test"
	logger := zap.NewExample().Sugar()
	r := &mockReporter{}
	return &ref{
		ce:       ce,
		source:   source,
		logger:   logger,
		reporter: r,
	}, ce
}

func validateMetric(t *testing.T, reporter source.StatsReporter, want int) {
	if mockReporter, ok := reporter.(*mockReporter); !ok {
		t.Errorf("reporter is not a mockReporter")
	} else if mockReporter.eventCount != want {
		t.Errorf("Expected %d for metric, got %d", want, mockReporter.eventCount)
	}
}
