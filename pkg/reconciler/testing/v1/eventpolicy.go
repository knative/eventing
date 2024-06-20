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

package testing

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/pkg/apis"
)

// EventPolicyOption enables further configuration of an EventPolicy.
type EventPolicyOption func(*v1alpha1.EventPolicy)

// NewEventPolicy creates a EventPolicy with EventPolicyOptions.
func NewEventPolicy(name, namespace string, o ...EventPolicyOption) *v1alpha1.EventPolicy {
	ep := &v1alpha1.EventPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	for _, opt := range o {
		opt(ep)
	}
	ep.SetDefaults(context.Background())

	return ep
}

func WithInitEventPolicyConditions(et *v1alpha1.EventPolicy) {
	et.Status.InitializeConditions()
}

func WithReadyEventPolicyCondition(ep *v1alpha1.EventPolicy) {
	ep.Status.Conditions = []apis.Condition{
		{
			Type:   v1alpha1.EventPolicyConditionReady,
			Status: corev1.ConditionTrue,
		},
	}
}

func WithUnreadyEventPolicyCondition(ep *v1alpha1.EventPolicy) {
	ep.Status.Conditions = []apis.Condition{
		{
			Type:   v1alpha1.EventPolicyConditionReady,
			Status: corev1.ConditionFalse,
		},
	}
}

func WithEventPolicyTo(tos ...v1alpha1.EventPolicySpecTo) EventPolicyOption {
	return func(ep *v1alpha1.EventPolicy) {
		ep.Spec.To = append(ep.Spec.To, tos...)
	}
}

func WithEventPolicyToRef(ref v1alpha1.EventPolicyToReference) EventPolicyOption {
	return func(ep *v1alpha1.EventPolicy) {
		ep.Spec.To = append(ep.Spec.To, v1alpha1.EventPolicySpecTo{
			Ref: &ref,
		})
	}
}

func WithEventPolicyFrom(froms ...v1alpha1.EventPolicySpecFrom) EventPolicyOption {
	return func(ep *v1alpha1.EventPolicy) {
		ep.Spec.From = append(ep.Spec.From, froms...)
	}
}

func WithEventPolicyLabels(labels map[string]string) EventPolicyOption {
	return func(ep *v1alpha1.EventPolicy) {
		ep.ObjectMeta.Labels = labels
	}
}

func WithEventPolicyOwnerReference(ownerRef metav1.OwnerReference) EventPolicyOption {
	return func(ep *v1alpha1.EventPolicy) {
		ep.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
			ownerRef,
		}
	}
}
