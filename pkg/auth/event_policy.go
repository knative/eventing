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
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	listerseventingv1alpha1 "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	"knative.dev/pkg/resolver"
)

// GetEventPoliciesForResource returns the applying EventPolicies for a given resource
func GetEventPoliciesForResource(lister listerseventingv1alpha1.EventPolicyLister, resourceGVK schema.GroupVersionKind, resourceObjectMeta metav1.ObjectMeta) ([]*v1alpha1.EventPolicy, error) {
	policies, err := lister.EventPolicies(resourceObjectMeta.GetNamespace()).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list eventpolicies: %w", err)
	}

	relevantPolicies := []*v1alpha1.EventPolicy{}

	for _, policy := range policies {
		if len(policy.Spec.To) == 0 {
			// policy applies to all resources in namespace
			relevantPolicies = append(relevantPolicies, policy)
		}

		for _, to := range policy.Spec.To {
			if to.Ref != nil {
				refGV, err := schema.ParseGroupVersion(to.Ref.APIVersion)
				if err != nil {
					return nil, fmt.Errorf("cannot split apiVersion into group and version: %s", to.Ref.APIVersion)
				}

				if strings.EqualFold(to.Ref.Name, resourceObjectMeta.GetName()) &&
					strings.EqualFold(refGV.Group, resourceGVK.Group) &&
					strings.EqualFold(to.Ref.Kind, resourceGVK.Kind) {

					relevantPolicies = append(relevantPolicies, policy)
					break // no need to check the other .spec.to's from this policy
				}
			}

			if to.Selector != nil {
				selectorGV, err := schema.ParseGroupVersion(to.Selector.APIVersion)
				if err != nil {
					return nil, fmt.Errorf("cannot split apiVersion into group and version: %s", to.Selector.APIVersion)
				}

				if strings.EqualFold(selectorGV.Group, resourceGVK.Group) &&
					strings.EqualFold(to.Selector.Kind, resourceGVK.Kind) {

					selector, err := metav1.LabelSelectorAsSelector(to.Selector.LabelSelector)
					if err != nil {
						return nil, fmt.Errorf("failed to parse selector: %w", err)
					}

					if selector.Matches(labels.Set(resourceObjectMeta.Labels)) {
						relevantPolicies = append(relevantPolicies, policy)
						break // no need to check the other .spec.to's from this policy
					}
				}
			}
		}
	}

	return relevantPolicies, nil
}

// ResolveSubjects returns the OIDC service accounts names for the objects referenced in the EventPolicySpecFrom.
func ResolveSubjects(resolver *resolver.AuthenticatableResolver, eventPolicy *v1alpha1.EventPolicy) ([]string, error) {
	allSAs := []string{}
	for _, from := range eventPolicy.Spec.From {
		if from.Ref != nil {
			sas, err := resolveSubjectsFromReference(resolver, *from.Ref, eventPolicy)
			if err != nil {
				return nil, fmt.Errorf("could not resolve subjects from reference: %w", err)
			}
			allSAs = append(allSAs, sas...)
		} else if from.Sub != nil {
			allSAs = append(allSAs, *from.Sub)
		}
	}

	return allSAs, nil
}

func resolveSubjectsFromReference(resolver *resolver.AuthenticatableResolver, reference v1alpha1.EventPolicyFromReference, trackingEventPolicy *v1alpha1.EventPolicy) ([]string, error) {
	authStatus, err := resolver.AuthStatusFromObjectReference(&corev1.ObjectReference{
		APIVersion: reference.APIVersion,
		Kind:       reference.Kind,
		Namespace:  reference.Namespace,
		Name:       reference.Name,
	}, trackingEventPolicy)

	if err != nil {
		return nil, fmt.Errorf("could not resolve auth status: %w", err)
	}

	objSAs := authStatus.ServiceAccountNames
	if authStatus.ServiceAccountName != nil {
		objSAs = append(objSAs, *authStatus.ServiceAccountName)
	}

	objFullSANames := make([]string, 0, len(objSAs))
	for _, sa := range objSAs {
		objFullSANames = append(objFullSANames, fmt.Sprintf("system:serviceaccount:%s:%s", reference.Namespace, sa))
	}

	return objFullSANames, nil
}

// SubjectContained checks if the given sub is contained in the list of allowedSubs
// or if it matches a prefix pattern in subs (e.g. system:serviceaccounts:my-ns:*)
func SubjectContained(sub string, allowedSubs []string) bool {
	for _, s := range allowedSubs {
		if strings.EqualFold(s, sub) {
			return true
		}

		if strings.HasSuffix(s, "*") &&
			strings.HasPrefix(sub, strings.TrimSuffix(s, "*")) {
			return true
		}
	}

	return false
}
