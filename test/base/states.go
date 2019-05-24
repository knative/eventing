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
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/kmeta"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
)

// ResourceReadyChecker returns a checker function that can check if the given resource is ready.
func ResourceReadyChecker(dynamicClient dynamic.Interface) func(...kmeta.OwnerRefable) (bool, error) {
	return func(objs ...kmeta.OwnerRefable) (bool, error) {
		for _, o := range objs {
			if isReady, err := isResourceReady(dynamicClient, o); !isReady || err != nil {
				return isReady, err
			}
		}
		return true, nil
	}
}

// isResourceReady is a generic method to check if the resource that implements KResource duck is ready.
func isResourceReady(dynamicClient dynamic.Interface, obj kmeta.OwnerRefable) (bool, error) {
	// get the resource's name, namespace and gvr
	name := obj.GetObjectMeta().GetName()
	namespace := obj.GetObjectMeta().GetNamespace()
	gvk := obj.GetGroupVersionKind()
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	// use the helper functions to convert the resource to a KResource duck
	tif := &duck.TypedInformerFactory{Client: dynamicClient, Type: &duckv1alpha1.KResource{}}
	_, lister, err := tif.Get(gvr)
	if err != nil {
		return false, err
	}
	untyped, err := lister.ByNamespace(namespace).Get(name)
	if err != nil {
		return false, err
	}
	kr := untyped.(*duckv1alpha1.KResource)
	return kr.Status.GetCondition(duckv1alpha1.ConditionReady).IsTrue(), nil
}
