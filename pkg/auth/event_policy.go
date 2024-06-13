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
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	eventingv1alpha1 "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	"knative.dev/pkg/kmeta"
)

func GetEventPoliciesForResource(lister eventingv1alpha1.EventPolicyLister, resource kmeta.Accessor) ([]*v1alpha1.EventPolicy, error) {
	policies, err := lister.EventPolicies(resource.GetNamespace()).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list eventpolicies: %w", err)
	}

	relevantPolicies := []*v1alpha1.EventPolicy{}
	resourceAPIVersion := fmt.Sprintf("%s/%s", resource.GroupVersionKind().Group, resource.GroupVersionKind().Version)

	for _, policy := range policies {
		for _, to := range policy.Spec.To {

			if to.Ref != nil &&
				to.Ref.Name == resource.GetName() &&
				to.Ref.APIVersion == resourceAPIVersion &&
				to.Ref.Kind == resource.GroupVersionKind().Kind {

				relevantPolicies = append(relevantPolicies, policy)
				break // no need to check the other .spec.to's from this policy
			}

			if to.Selector != nil &&
				to.Selector.APIVersion == resourceAPIVersion &&
				to.Selector.Kind == resource.GroupVersionKind().Kind {

				requiredLabelsMatch := true
				for requiredLabelKey, requiredLabelVal := range to.Selector.MatchLabels {
					resourceLabels := resource.GetLabels()
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
