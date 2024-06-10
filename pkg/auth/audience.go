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
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GetAudience returns the audience string for the given object in the format <group>/<kind>/<namespace>/<name>
func GetAudience(gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta) string {
	aud := fmt.Sprintf("%s/%s/%s/%s", gvk.Group, gvk.Kind, objectMeta.Namespace, objectMeta.Name)

	return strings.ToLower(aud)
}

// GetAudienceDirect returns the audience string for the given object in the format <group>/<kind>/<namespace>/<name>
func GetAudienceDirect(gvk schema.GroupVersionKind, ns, name string) string {
	aud := fmt.Sprintf("%s/%s/%s/%s", gvk.Group, gvk.Kind, ns, name)

	return strings.ToLower(aud)
}
