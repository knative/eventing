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

package reconciler

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
)

// enqueueApplyingResourcesOfEventPolicy checks if the given GVK is referenced in the given EventPolicy.
// If so, it enqueues it into the enqueueFn().
func enqueueApplyingResourcesOfEventPolicy(indexer cache.Indexer, gvk schema.GroupVersionKind, policyObj interface{}, enqueueFn func(key types.NamespacedName)) {
	eventPolicy, ok := policyObj.(*v1alpha1.EventPolicy)
	if !ok {
		return
	}

	for _, to := range eventPolicy.Spec.To {
		if to.Ref != nil {
			toGV, err := schema.ParseGroupVersion(to.Ref.APIVersion)
			if err != nil {
				continue
			}

			if strings.EqualFold(toGV.Group, gvk.Group) &&
				strings.EqualFold(to.Ref.Kind, gvk.Kind) {

				enqueueFn(types.NamespacedName{
					Namespace: eventPolicy.Namespace,
					Name:      to.Ref.Name,
				})
			}
		}

		if to.Selector != nil {
			selectorGV, err := schema.ParseGroupVersion(to.Selector.APIVersion)
			if err != nil {
				continue
			}

			if strings.EqualFold(selectorGV.Group, gvk.Group) &&
				strings.EqualFold(to.Selector.Kind, gvk.Kind) {

				selector, err := metav1.LabelSelectorAsSelector(to.Selector.LabelSelector)
				if err != nil {
					continue
				}

				resources := []metav1.Object{}
				err = cache.ListAllByNamespace(indexer, eventPolicy.Namespace, selector, func(i interface{}) {
					resources = append(resources, i.(metav1.Object))
				})
				if err != nil {
					continue
				}

				for _, resource := range resources {
					enqueueFn(types.NamespacedName{
						Namespace: resource.GetNamespace(),
						Name:      resource.GetName(),
					})
				}
			}
		}
	}
}

// EventPolicyEventHandler returns an ResourceEventHandler, which enqueues the referencing resources of the EventPolicy
// if the EventPolicy was referencing or got updated and now is referencing the resource of the given GVK.
func EventPolicyEventHandler(indexer cache.Indexer, gvk schema.GroupVersionKind, enqueueFn func(key types.NamespacedName)) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			enqueueApplyingResourcesOfEventPolicy(indexer, gvk, obj, enqueueFn)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			// Here we need to check if the old or the new EventPolicy was referencing the given GVK

			// make sure, we enqueue the keys only once
			toEnqueue := map[types.NamespacedName]struct{}{}
			addToEnqueueList := func(key types.NamespacedName) {
				toEnqueue[key] = struct{}{}
			}
			enqueueApplyingResourcesOfEventPolicy(indexer, gvk, oldObj, addToEnqueueList)
			enqueueApplyingResourcesOfEventPolicy(indexer, gvk, newObj, addToEnqueueList)

			for k := range toEnqueue {
				enqueueFn(k)
			}
		},
		DeleteFunc: func(obj interface{}) {
			enqueueApplyingResourcesOfEventPolicy(indexer, gvk, obj, enqueueFn)
		},
	}
}
