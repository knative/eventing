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

package testing

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// SubscriptionOption enables further configuration of a Subscription.
type SubscriptionOption func(*v1alpha1.Subscription)

// NewSubscription creates a Subscription with SubscriptionOptions
func NewSubscription(name, namespace string, so ...SubscriptionOption) *v1alpha1.Subscription {
	s := &v1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range so {
		opt(s)
	}
	s.SetDefaults(context.Background())
	return s
}

// NewSubscriptionWithoutNamespace creates a Subscription with SubscriptionOptions but without a specific namespace
func NewSubscriptionWithoutNamespace(name string, so ...SubscriptionOption) *v1alpha1.Subscription {
	s := &v1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, opt := range so {
		opt(s)
	}
	s.SetDefaults(context.Background())
	return s
}

func WithSubscriptionUID(uid types.UID) SubscriptionOption {
	return func(s *v1alpha1.Subscription) {
		s.UID = uid
	}
}

func WithSubscriptionGeneration(gen int64) SubscriptionOption {
	return func(s *v1alpha1.Subscription) {
		s.Generation = gen
	}
}

func WithSubscriptionGenerateName(generateName string) SubscriptionOption {
	return func(c *v1alpha1.Subscription) {
		c.ObjectMeta.GenerateName = generateName
	}
}

// WithInitSubscriptionConditions initializes the Subscriptions's conditions.
func WithInitSubscriptionConditions(s *v1alpha1.Subscription) {
	s.Status.InitializeConditions()
}

func WithSubscriptionReady(s *v1alpha1.Subscription) {
	s.Status = *eventingv1alpha1.TestHelper.ReadySubscriptionStatus()
}

// TODO: this can be a runtime object
func WithSubscriptionDeleted(s *v1alpha1.Subscription) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	s.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithSubscriptionOwnerReferences(ownerReferences []metav1.OwnerReference) SubscriptionOption {
	return func(c *v1alpha1.Subscription) {
		c.ObjectMeta.OwnerReferences = ownerReferences
	}
}

func WithSubscriptionLabels(labels map[string]string) SubscriptionOption {
	return func(c *v1alpha1.Subscription) {
		c.ObjectMeta.Labels = labels
	}
}

func WithSubscriptionChannel(gvk metav1.GroupVersionKind, name string) SubscriptionOption {
	return func(s *v1alpha1.Subscription) {
		s.Spec.Channel = corev1.ObjectReference{
			APIVersion: apiVersion(gvk),
			Kind:       gvk.Kind,
			Name:       name,
		}
	}
}

func WithSubscriptionSubscriberRef(gvk metav1.GroupVersionKind, name string) SubscriptionOption {
	return func(s *v1alpha1.Subscription) {
		s.Spec.Subscriber = &duckv1beta1.Destination{
			Ref: &corev1.ObjectReference{
				APIVersion: apiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithSubscriptionPhysicalSubscriptionSubscriber(uri string) SubscriptionOption {
	return func(s *v1alpha1.Subscription) {
		u, err := apis.ParseURL(uri)
		if err != nil {
			panic(fmt.Sprintf("failed to parse URL: %s", err))
		}
		s.Status.PhysicalSubscription.SubscriberURI = u
	}
}

func WithSubscriptionPhysicalSubscriptionReply(uri string) SubscriptionOption {
	return func(s *v1alpha1.Subscription) {
		u, err := apis.ParseURL(uri)
		if err != nil {
			panic(fmt.Sprintf("failed to parse URL: %s", err))
		}
		s.Status.PhysicalSubscription.ReplyURI = u
	}
}

func WithSubscriptionFinalizers(finalizers ...string) SubscriptionOption {
	return func(s *v1alpha1.Subscription) {
		s.Finalizers = finalizers
	}
}

func MarkSubscriptionReady(s *v1alpha1.Subscription) {
	s.Status.MarkChannelReady()
	s.Status.MarkReferencesResolved()
	s.Status.MarkAddedToChannel()
}

func WithSubscriptionReferencesNotResolved(reason, msg string) SubscriptionOption {
	return func(s *v1alpha1.Subscription) {
		s.Status.MarkReferencesNotResolved(reason, msg)
	}
}

func WithSubscriptionReply(gvk metav1.GroupVersionKind, name string) SubscriptionOption {
	return func(s *v1alpha1.Subscription) {
		s.Spec.Reply = &v1alpha1.ReplyStrategy{
			Destination: &duckv1beta1.Destination{
				DeprecatedAPIVersion: apiVersion(gvk),
				DeprecatedKind:       gvk.Kind,
				DeprecatedName:       name,
			},
		}
	}
}

func WithSubscriptionReplyNotDeprecated(gvk metav1.GroupVersionKind, name string) SubscriptionOption {
	return func(s *v1alpha1.Subscription) {
		s.Spec.Reply = &v1alpha1.ReplyStrategy{
			Destination: &duckv1beta1.Destination{
				Ref: &corev1.ObjectReference{
					APIVersion: apiVersion(gvk),
					Kind:       gvk.Kind,
					Name:       name,
				},
			},
		}
	}
}

func WithSubscriptionReplyDeprecated() SubscriptionOption {
	return func(s *v1alpha1.Subscription) {
		s.Status.MarkReplyDeprecatedRef("ReplyFieldsDeprecated", "Using deprecated object ref fields when specifying spec.reply. Update to spec.reply.ref. These will be removed in 0.11")
	}
}
