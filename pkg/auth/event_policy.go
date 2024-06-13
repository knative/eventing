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
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	listerseventingv1alpha1 "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"strings"
)

// GetEventPoliciesForResource returns the applying EventPolicies for a given resource
func GetEventPoliciesForResource(lister listerseventingv1alpha1.EventPolicyLister, resourceGVK schema.GroupVersionKind, resourceObjectMeta metav1.ObjectMeta) ([]*v1alpha1.EventPolicy, error) {
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

// ResolveSubjects returns the OIDC service accounts names for the objects referenced in the EventPolicySpecFrom.
func ResolveSubjects(ctx context.Context, dynamicClient dynamic.Interface, froms []v1alpha1.EventPolicySpecFrom, namespace string) ([]string, error) {
	allSAs := []string{}
	for _, from := range froms {
		if from.Ref != nil {
			sas, err := resolveSubjectsFromReference(ctx, dynamicClient, *from.Ref, namespace)
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

func resolveSubjectsFromReference(ctx context.Context, dynamicClient dynamic.Interface, reference v1alpha1.EventPolicyFromReference, namespace string) ([]string, error) {
	parts := strings.Split(reference.APIVersion, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("cannot split apiVersion into group and version: %s", reference.APIVersion)
	}

	gvr, _ := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{
		Group:   parts[0],
		Version: parts[1],
		Kind:    reference.Kind,
	})

	unstructured, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, reference.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to resolve reference: %w", err)
	}

	return getOIDCSAsFromUnstructured(unstructured)
}

func getOIDCSAsFromUnstructured(unstructured *unstructured.Unstructured) ([]string, error) {
	type AuthenticatableType struct {
		metav1.TypeMeta   `json:",inline"`
		metav1.ObjectMeta `json:"metadata,omitempty"`

		Status struct {
			Auth *duckv1.AuthStatus `json:"auth,omitempty"`
		} `json:"status"`
	}

	obj := &AuthenticatableType{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.Object, obj); err != nil {
		return nil, fmt.Errorf("error from DefaultUnstructured.Dynamiconverter: %w", err)
	}

	if obj.Status.Auth == nil || (obj.Status.Auth.ServiceAccountName == nil && len(obj.Status.Auth.ServiceAccountNames) == 0) {
		return nil, fmt.Errorf("resource does not have an OIDC service account set")
	}

	objSAs := obj.Status.Auth.ServiceAccountNames
	if obj.Status.Auth.ServiceAccountName != nil {
		objSAs = append(objSAs, *obj.Status.Auth.ServiceAccountName)
	}

	objFullSANames := make([]string, 0, len(objSAs))
	for _, sa := range objSAs {
		objFullSANames = append(objFullSANames, fmt.Sprintf("system:serviceaccount:%s:%s", obj.GetNamespace(), sa))
	}

	return objFullSANames, nil
}
