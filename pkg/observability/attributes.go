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
	"go.opentelemetry.io/otel/attribute"
	"knative.dev/pkg/observability/attributekey"
)

// K8sAttributes generates Kubernetes trace attributes for the object of the
// given name in the given namespace. resource identifies the object type
// as <singular>.<group>
func K8sAttributes(name, namespace, resource string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attributekey.String("k8s." + resource + ".name").With(name),
		attributekey.String("k8s." + resource + ".namespace").With(namespace),
	}
}
