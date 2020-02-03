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
	"time"

	"k8s.io/apimachinery/pkg/types"
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
	client.CreateConfigMapOrFail(testingCM1, eventingNamespace, map[string]string{
		"firstdata":  "data1",
		"seconddata": "data2",
	})
	client.CreateConfigMapOrFail(testingCM2, eventingNamespace, map[string]string{
		"thirddata":  "data3",
		"fourthdata": "data4",
	})

	// CMP copies all required configmaps from 'eventingNamespace' to current client namespace
	client.CreateConfigMapPropagationOrFail(defaultCMP)

	// Check if copy configmap exists and contains the same data as original configmap
	if err := client.CheckConfigMapsEqual(eventingNamespace, defaultCMP, 4*time.Second, testingCM1, testingCM2); err != nil {
		t.Fatalf("Failed to check copy configamp contains the same data as original configmap: %v", err)
	}

	payload := []patchUInt32Value{{
		Op:   "remove",
		Path: "/data/firstdata",
	}}
	payloadBytes, _ := json.Marshal(payload)

	// Remove one data key from copy configmap testingCM1
	if _, err := client.Kube.Kube.CoreV1().ConfigMaps(client.Namespace).Patch(defaultCMP+"-"+testingCM1, types.JSONPatchType, payloadBytes); err != nil {
		t.Fatalf("Failed to patch copy configmap: %v", err)
	}
	// Check if copy configmap will revert back
	if err := client.CheckConfigMapsEqual(eventingNamespace, defaultCMP, 2*time.Second, testingCM1); err != nil {
		t.Fatalf("Failed to check copy configmap will revert back: %v", err)
	}

	// Remove one data key from original configmap
	if _, err := client.Kube.Kube.CoreV1().ConfigMaps(eventingNamespace).Patch(testingCM1, types.JSONPatchType, payloadBytes); err != nil {
		t.Fatalf("Failed to patch original configmap: %v", err)
	}
	// Check if copy configmap will update after original configmap changes
	if err := client.CheckConfigMapsEqual(eventingNamespace, defaultCMP, 2*time.Second, testingCM1); err != nil {
		t.Fatalf("Failed to check if copy configmap will update after original configmap changes: %v", err)
	}
	//time.Sleep(time.Minute)
}

type patchUInt32Value struct {
	Op   string `json:"op"`
	Path string `json:"path"`
}
