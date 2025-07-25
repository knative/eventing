/*
Copyright 2022 The Knative Authors

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

package observability

import (
	"reflect"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"knative.dev/pkg/observability/attributekey"
)

func TestK8sAttributes(t *testing.T) {

	tests := []struct {
		name         string
		namespace    string
		resourceType string
		want         []attribute.KeyValue
	}{
		{
			name:         "foo",
			namespace:    "default",
			resourceType: "resource.k8s.io",

			want: []attribute.KeyValue{
				attributekey.String("k8s.resource.k8s.io.name").With("foo"),
				attributekey.String("k8s.resource.k8s.io.namespace").With("default"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := K8sAttributes(tt.name, tt.namespace, tt.resourceType); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("K8sAttributes() = %v, want %v", got, tt.want)
			}
		})
	}
}
