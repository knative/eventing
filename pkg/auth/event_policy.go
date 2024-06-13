/*
Copyright 2024 The Knative Authors

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	eventingv1alpha1 "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
)

// GetEventPoliciesForResource returns the applying EventPolicies for a given resource
func GetEventPoliciesForResource(lister eventingv1alpha1.EventPolicyLister, resourceGVK schema.GroupVersionKind, resourceObjectMeta metav1.ObjectMeta) ([]*v1alpha1.EventPolicy, error) {
	policies, err := lister.EventPolicies(resourceObjectMeta.GetNamespace()).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list eventpolicies: %w", err)
	}

	relevantPolicies := []*v1alpha1.EventPolicy{}
	resourceAPIVersion := fmt.Sprintf("%s/%s", resourceGVK.Group, resourceGVK.Version)

	for _, policy := range policies {
		for _, to := range policy.Spec.To {

			if to.Ref != nil &&
				to.Ref.Name == resourceObjectMeta.GetName() &&
				to.Ref.APIVersion == resourceAPIVersion &&
				to.Ref.Kind == resourceGVK.Kind {

				relevantPolicies = append(relevantPolicies, policy)
				break // no need to check the other .spec.to's from this policy
			}

			if to.Selector != nil &&
				to.Selector.APIVersion == resourceAPIVersion &&
				to.Selector.Kind == resourceGVK.Kind {

				requiredLabelsMatch := true
				for requiredLabelKey, requiredLabelVal := range to.Selector.MatchLabels {
					resourceLabels := resourceObjectMeta.GetLabels()
					if resourceLabels[requiredLabelKey] != requiredLabelVal {
						// required label not found
						requiredLabelsMatch = false
						break
					}
				}

				if requiredLabelsMatch {
					relevantPolicies = append(relevantPolicies, policy)
					break // no need to check the other .spec.to's from this policy
				}
			}
		}

		if len(policy.Spec.To) == 0 {
			// policy applies to all resources in namespace
			relevantPolicies = append(relevantPolicies, policy)
		}
	}

	return relevantPolicies, nil
}
