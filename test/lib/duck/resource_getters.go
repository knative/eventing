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

package duck

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"

	"knative.dev/eventing/test/lib/resources"
)

// GetGenericObject returns a generic object representing a Kubernetes resource.
// Callers can cast this returned object to other objects that implement the corresponding duck-type.
func GetGenericObject(
	dynamicClient dynamic.Interface,
	obj *resources.MetaResource,
	rtype apis.Listable,
) (runtime.Object, error) {
	// get the resource's namespace and gvr
	gvr, _ := meta.UnsafeGuessKindToResource(obj.GroupVersionKind())
	u, err := dynamicClient.Resource(gvr).Namespace(obj.Namespace).Get(obj.Name, metav1.GetOptions{})

	if err != nil {
		return nil, err
	}

	res := rtype.DeepCopyObject()
	if err := duck.FromUnstructured(u, res); err != nil {
		return nil, err
	}

	return res, nil
}

// GetGenericObjectList returns a generic object list representing a list of Kubernetes resource.
func GetGenericObjectList(
	dynamicClient dynamic.Interface,
	objList *resources.MetaResourceList,
	rtype apis.Listable,
) ([]runtime.Object, error) {
	// get the resource's namespace and gvr
	gvr, _ := meta.UnsafeGuessKindToResource(objList.GroupVersionKind())
	ul, err := dynamicClient.Resource(gvr).Namespace(objList.Namespace).List(metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	objs := make([]runtime.Object, 0, len(ul.Items))
	for _, u := range ul.Items {
		res := rtype.DeepCopyObject()
		if err := duck.FromUnstructured(&u, res); err != nil {
			return nil, err
		}
		objs = append(objs, res)
	}

	return objs, nil
}
