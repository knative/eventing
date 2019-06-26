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

// This file contains functions which get actual resources given the meta resource.

package base

import (
	"github.com/knative/eventing/test/base/resources"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
)

// GetGenericObject returns a generic object representing a Kubernetes resource.
// Callers can cast this returned object to other objects that implement the corresponding duck-type.
func GetGenericObject(
	dynamicClient dynamic.Interface,
	obj *resources.MetaResource,
	rtype apis.Listable,
) (runtime.Object, error) {
	lister, err := getGenericLister(dynamicClient, obj.GroupVersionKind(), obj.Namespace, rtype)
	if err != nil {
		return nil, err
	}
	return lister.Get(obj.Name)
}

// GetGenericObjectList returns a generic object list representing a list of Kubernetes resource.
func GetGenericObjectList(
	dynamicClient dynamic.Interface,
	objList *resources.MetaResourceList,
	rtype apis.Listable,
) ([]runtime.Object, error) {
	lister, err := getGenericLister(dynamicClient, objList.GroupVersionKind(), objList.Namespace, rtype)
	if err != nil {
		return nil, err
	}
	return lister.List(labels.Everything())
}

// getGenericLister returns a GenericNamespacedLister, which can be used to get resources in the namespace.
func getGenericLister(
	dynamicClient dynamic.Interface,
	gvk schema.GroupVersionKind,
	namespace string,
	rtype apis.Listable,
) (cache.GenericNamespaceLister, error) {
	// get the resource's namespace and gvr
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
	return lister.ByNamespace(namespace), nil
}
