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

package resources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MetaResource includes necessary meta data to retrieve the generic Kubernetes resource.
type MetaResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// MetaResourceList includes necessary meta data to retrieve the generic Kubernetes resource list.
type MetaResourceList struct {
	metav1.TypeMeta `json:",inline"`
	Namespace       string
}

// NewMetaResource returns a MetaResource built from the given name, namespace and typemeta.
func NewMetaResource(name, namespace string, typemeta *metav1.TypeMeta) *MetaResource {
	return &MetaResource{
		TypeMeta: *typemeta,
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
}

// NewMetaResourceList returns a MetaResourceList built from the given namespace and typemeta.
func NewMetaResourceList(namespace string, typemeta *metav1.TypeMeta) *MetaResourceList {
	return &MetaResourceList{
		TypeMeta:  *typemeta,
		Namespace: namespace,
	}
}
