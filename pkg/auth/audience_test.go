/*
Copyright 2023 The Knative Authors

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

package auth

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

func TestGetAudience(t *testing.T) {

	tests := []struct {
		name       string
		gvk        schema.GroupVersionKind
		objectMeta metav1.ObjectMeta
		want       string
	}{
		{
			name: "should return audience in correct format",
			gvk: schema.GroupVersionKind{
				Group:   "group",
				Version: "version",
				Kind:    "kind",
			},
			objectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
			want: "group/kind/namespace/name",
		},
		{
			name: "should return audience in lower case",
			gvk:  v1.SchemeGroupVersion.WithKind("Broker"),
			objectMeta: metav1.ObjectMeta{
				Name:      "my-Broker",
				Namespace: "my-Namespace",
			},
			want: "eventing.knative.dev/broker/my-namespace/my-broker",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetAudience(tt.gvk, tt.objectMeta); got != tt.want {
				t.Errorf("GetAudience() = %v, want %v", got, tt.want)
			}
		})
	}
}
