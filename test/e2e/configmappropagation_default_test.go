// +build e2e

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

package e2e

import (
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"

	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
)

func TestDefaultConfigMapPropagation(t *testing.T) {
	const (
		defaultCMP        = "eventing"
		testingCM1        = "config-testing1"
		testingCM2        = "config-testing2"
		eventingNamespace = "knative-eventing"
	)
	client := setup(t, true)
	defer tearDown(client)

	// Create two new configmaps
	if _, err := client.Kube.Kube.CoreV1().ConfigMaps(eventingNamespace).Create(resources.ConfigMap(testingCM1, map[string]string{
		"firstdata":  "data1",
		"seconddata": "data2",
	})); err != nil {
		t.Fatalf("Failed to create configmap "+testingCM1+": %v", err)
	}

	if _, err := client.Kube.Kube.CoreV1().ConfigMaps(eventingNamespace).Create(resources.ConfigMap(testingCM2, map[string]string{
		"thirddata":  "data3",
		"fourthdata": "data4",
	})); err != nil {
		t.Fatalf("Failed to create configmap "+testingCM2+": %v", err)
	}

	defer deleteConfigMap(client, eventingNamespace, testingCM1, testingCM2)

	// Check if cmp successfully copies all required configmap
	client.CreateConfigMapPropagationOrFail(defaultCMP)

	if err := client.ConfigMapExists(client.Namespace, defaultCMP+"-"+testingCM1, defaultCMP+"-"+testingCM2); err != nil {
		t.Fatalf("Failed to check the existence for all copied configmaps: %v", err)
	}
	// Check if copy configmap contains the same data as original configmap
	if err := client.ConfigMapEqual(eventingNamespace, defaultCMP, testingCM1, testingCM2); err != nil {
		t.Fatalf("Failed to check copy configamp contains the same data as original configmap: %v", err)
	}

	payload := []patchUInt32Value{{
		Op:   "remove",
		Path: "/data/firstdata",
	}}
	payloadBytes, _ := json.Marshal(payload)

	// Check if copy configmap will revert after updated
	if _, err := client.Kube.Kube.CoreV1().ConfigMaps(client.Namespace).Patch(defaultCMP+"-"+testingCM1, types.JSONPatchType, payloadBytes); err != nil {
		t.Fatalf("Failed to patch copy configmap: %v", err)
	}
	if err := client.ConfigMapEqual(eventingNamespace, defaultCMP, testingCM1); err != nil {
		t.Fatalf("Failed to check copy configmap will revert after updated: %v", err)
	}

	// Check if copy configmap will update after original configmap changes
	// Remove one data key from original map
	if _, err := client.Kube.Kube.CoreV1().ConfigMaps(eventingNamespace).Patch(testingCM1, types.JSONPatchType, payloadBytes); err != nil {
		t.Fatalf("Failed to patch original configmap: %v", err)
	}
	if err := client.ConfigMapEqual(eventingNamespace, defaultCMP, testingCM1); err != nil {
		t.Fatalf("Failed to check if copy configmap will update after original configmap changes: %v", err)
	}
}

type patchUInt32Value struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value uint32 `json:"value"`
}

func deleteConfigMap(client *lib.Client, namespace string, names ...string) {
	if names != nil {
		for _, name := range names {
			client.Kube.Kube.CoreV1().ConfigMaps(namespace).Delete(name, &metav1.DeleteOptions{})
		}
	}
}
