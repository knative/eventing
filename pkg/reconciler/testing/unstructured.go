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

package testing

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UnstructuredOption enables further configuration of a Unstructured.
type UnstructuredOption func(*unstructured.Unstructured)

// NewUnstructured creates a unstructured.Unstructured with UnstructuredOption
func NewUnstructured(gvk metav1.GroupVersionKind, name, namespace string, uo ...UnstructuredOption) *unstructured.Unstructured {
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion(gvk),
			"kind":       gvk.Kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
			"spec":   map[string]interface{}{},
			"status": map[string]interface{}{},
		},
	}
	for _, opt := range uo {
		opt(u)
	}
	return u
}

func WithUnstructuredAddressable(hostname string) UnstructuredOption {
	return func(u *unstructured.Unstructured) {
		status, ok := u.Object["status"].(map[string]interface{})
		if ok {
			status["address"] = map[string]interface{}{
				"hostname": hostname,
				"url":      fmt.Sprintf("http://%s", hostname),
			}
		}
	}
}

func apiVersion(gvk metav1.GroupVersionKind) string {
	groupVersion := gvk.Version
	if gvk.Group != "" {
		groupVersion = gvk.Group + "/" + gvk.Version
	}
	return groupVersion
}
