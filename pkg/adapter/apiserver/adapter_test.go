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
	kncetesting "github.com/knative/eventing/pkg/kncloudevents/testing"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"testing"
)

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

	return &resource{
		ce:     ce,
		source: source,
		logger: logger,
	}, ce
}

func makeRefAndTestingClient() (*ref, *kncetesting.TestCloudEventsClient) {
	ce := kncetesting.NewTestClient()
	source := "unit-test"
	logger := zap.NewExample().Sugar()

	return &ref{
		ce:     ce,
		source: source,
		logger: logger,
	}, ce
}
