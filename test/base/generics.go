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

package base

import (
	"github.com/knative/pkg/apis"
	"github.com/knative/pkg/apis/duck"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

// MetaResource includes necessary meta data to retrieve the generic Kubernetes resource.
type MetaResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// Meta returns a MetaResource built from the given name, namespace and kind.
func Meta(name, namespace, kind string) *MetaResource {
	return &MetaResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       kind,
			APIVersion: EventingAPIVersion,
		},
	}
}

// GetGenericObject returns a generic object representing a Kubernetes resource.
// Callers can cast this returned object to other objects that implement the corresponding duck-type.
func GetGenericObject(dynamicClient dynamic.Interface, obj *MetaResource, rtype apis.Listable) (runtime.Object, error) {
	// get the resource's name, namespace and gvr
	name := obj.Name
	namespace := obj.Namespace
	gvk := obj.GroupVersionKind()
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)

	stopChannel := make(chan struct{})
	// defer close the stopChannel to stop the informer created in tif.Get(gvr)
	defer close(stopChannel)
	// use the helper functions to convert the resource to the given duck-type
	tif := &duck.TypedInformerFactory{Client: dynamicClient, Type: rtype, StopChannel: stopChannel}
	_, lister, err := tif.Get(gvr)
	if err != nil {
		return nil, err
	}
	return lister.ByNamespace(namespace).Get(name)
}
