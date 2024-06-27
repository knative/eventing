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

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/feature"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

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

// GetApplyingResourcesOfEventPolicyForGK returns all applying resource names of GK of the given event policy.
// It returns only the names, as the resources are part of the same namespace as the event policy.
//
// This function is kind of the "inverse" of GetEventPoliciesForResource.
func GetApplyingResourcesOfEventPolicyForGK(eventPolicy *v1alpha1.EventPolicy, gk schema.GroupKind, gkIndexer cache.Indexer) ([]string, error) {
	applyingResources := map[string]struct{}{}

	if eventPolicy.Spec.To == nil {
		// empty .spec.to matches everything in namespace

		err := cache.ListAllByNamespace(gkIndexer, eventPolicy.Namespace, labels.Everything(), func(i interface{}) {
			name := i.(metav1.Object).GetName()
			applyingResources[name] = struct{}{}
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list all %s %s resources in %s: %w", gk.Group, gk.Kind, eventPolicy.Namespace, err)
		}
	} else {
		for _, to := range eventPolicy.Spec.To {
			if to.Ref != nil {
				toGV, err := schema.ParseGroupVersion(to.Ref.APIVersion)
				if err != nil {
					return nil, fmt.Errorf("could not parse group version of %q: %w", to.Ref.APIVersion, err)
				}

				if strings.EqualFold(toGV.Group, gk.Group) &&
					strings.EqualFold(to.Ref.Kind, gk.Kind) {

					applyingResources[to.Ref.Name] = struct{}{}
				}
			}

			if to.Selector != nil {
				selectorGV, err := schema.ParseGroupVersion(to.Selector.APIVersion)
				if err != nil {
					return nil, fmt.Errorf("could not parse group version of %q: %w", to.Selector.APIVersion, err)
				}

				if strings.EqualFold(selectorGV.Group, gk.Group) &&
					strings.EqualFold(to.Selector.Kind, gk.Kind) {

					selector, err := metav1.LabelSelectorAsSelector(to.Selector.LabelSelector)
					if err != nil {
						return nil, fmt.Errorf("could not parse label selector %v: %w", to.Selector.LabelSelector, err)
					}

					err = cache.ListAllByNamespace(gkIndexer, eventPolicy.Namespace, selector, func(i interface{}) {
						name := i.(metav1.Object).GetName()
						applyingResources[name] = struct{}{}
					})
					if err != nil {
						return nil, fmt.Errorf("could not list resources of GK in %q namespace for selector %v: %w", eventPolicy.Namespace, selector, err)
					}
				}
			}
		}
	}

	res := []string{}
	for name := range applyingResources {
		res = append(res, name)
	}
	return res, nil
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

func handleApplyingResourcesOfEventPolicy(eventPolicy *v1alpha1.EventPolicy, gk schema.GroupKind, indexer cache.Indexer, handlerFn func(key types.NamespacedName) error) error {
	applyingResources, err := GetApplyingResourcesOfEventPolicyForGK(eventPolicy, gk, indexer)
	if err != nil {
		return fmt.Errorf("could not get applying resources of eventpolicy: %w", err)
	}

	for _, resourceName := range applyingResources {
		err := handlerFn(types.NamespacedName{
			Namespace: eventPolicy.Namespace,
			Name:      resourceName,
		})

		if err != nil {
			return fmt.Errorf("could not handle resource %q: %w", resourceName, err)
		}
	}

	return nil
}

// EventPolicyEventHandler returns an ResourceEventHandler, which passes the referencing resources of the EventPolicy
// to the enqueueFn if the EventPolicy was referencing or got updated and now is referencing the resource of the given GVK.
func EventPolicyEventHandler(indexer cache.Indexer, gk schema.GroupKind, enqueueFn func(key types.NamespacedName)) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			eventPolicy, ok := obj.(*v1alpha1.EventPolicy)
			if !ok {
				return
			}

			handleApplyingResourcesOfEventPolicy(eventPolicy, gk, indexer, func(key types.NamespacedName) error {
				enqueueFn(key)
				return nil
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			// Here we need to check if the old or the new EventPolicy was referencing the given GVK
			oldEventPolicy, ok := oldObj.(*v1alpha1.EventPolicy)
			if !ok {
				return
			}
			newEventPolicy, ok := newObj.(*v1alpha1.EventPolicy)
			if !ok {
				return
			}

			// make sure, we handle the keys only once
			toHandle := map[types.NamespacedName]struct{}{}
			addToHandleList := func(key types.NamespacedName) error {
				toHandle[key] = struct{}{}
				return nil
			}

			handleApplyingResourcesOfEventPolicy(oldEventPolicy, gk, indexer, addToHandleList)
			handleApplyingResourcesOfEventPolicy(newEventPolicy, gk, indexer, addToHandleList)

			for k := range toHandle {
				enqueueFn(k)
			}
		},
		DeleteFunc: func(obj interface{}) {
			eventPolicy, ok := obj.(*v1alpha1.EventPolicy)
			if !ok {
				return
			}

			handleApplyingResourcesOfEventPolicy(eventPolicy, gk, indexer, func(key types.NamespacedName) error {
				enqueueFn(key)
				return nil
			})
		},
	}
}

type EventPolicyStatusMarker interface {
	MarkEventPoliciesFailed(reason, messageFormat string, messageA ...interface{})
	MarkEventPoliciesUnknown(reason, messageFormat string, messageA ...interface{})
	MarkEventPoliciesTrue()
	MarkEventPoliciesTrueWithReason(reason, messageFormat string, messageA ...interface{})
}

func UpdateStatusWithEventPolicies(featureFlags feature.Flags, status *eventingduckv1.AppliedEventPoliciesStatus, statusMarker EventPolicyStatusMarker, eventPolicyLister listerseventingv1alpha1.EventPolicyLister, gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta) error {
	status.Policies = nil

	applyingEvenPolicies, err := GetEventPoliciesForResource(eventPolicyLister, gvk, objectMeta)
	if err != nil {
		statusMarker.MarkEventPoliciesFailed("EventPoliciesGetFailed", "Failed to get applying event policies")
		return fmt.Errorf("unable to get applying event policies: %w", err)
	}

	if len(applyingEvenPolicies) > 0 {
		unreadyEventPolicies := []string{}
		for _, policy := range applyingEvenPolicies {
			if !policy.Status.IsReady() {
				unreadyEventPolicies = append(unreadyEventPolicies, policy.Name)
			} else {
				// only add Ready policies to the list
				status.Policies = append(status.Policies, eventingduckv1.AppliedEventPolicyRef{
					Name:       policy.Name,
					APIVersion: v1alpha1.SchemeGroupVersion.String(),
				})
			}
		}

		if len(unreadyEventPolicies) == 0 {
			statusMarker.MarkEventPoliciesTrue()
		} else {
			statusMarker.MarkEventPoliciesFailed("EventPoliciesNotReady", "event policies %s are not ready", strings.Join(unreadyEventPolicies, ", "))
		}
	} else {
		// we have no applying event policy. So we set the EP condition to True
		if featureFlags.IsOIDCAuthentication() {
			// in case of OIDC auth, we also set the message with the default authorization mode
			statusMarker.MarkEventPoliciesTrueWithReason("DefaultAuthorizationMode", "Default authz mode is %q", featureFlags[feature.AuthorizationDefaultMode])
		} else {
			// in case OIDC is disabled, we set EP condition to true too, but give some message that authz (EPs) require OIDC
			statusMarker.MarkEventPoliciesTrueWithReason("OIDCDisabled", "Feature %q must be enabled to support Authorization", feature.OIDCAuthentication)
		}
	}

	return nil
}
