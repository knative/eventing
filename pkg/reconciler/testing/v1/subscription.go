/*
Copyright 2020 The Knative Authors

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
	"errors"
	"time"

	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	v1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// SubscriptionOption enables further configuration of a Subscription.
type SubscriptionOption func(*v1.Subscription)

// NewSubscription creates a Subscription with SubscriptionOptions
func NewSubscription(name, namespace string, so ...SubscriptionOption) *v1.Subscription {
	s := &v1.Subscription{
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
func NewSubscriptionWithoutNamespace(name string, so ...SubscriptionOption) *v1.Subscription {
	s := &v1.Subscription{
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
	return func(s *v1.Subscription) {
		s.UID = uid
	}
}

func WithSubscriptionGeneration(gen int64) SubscriptionOption {
	return func(s *v1.Subscription) {
		s.Generation = gen
	}
}

func WithSubscriptionStatusObservedGeneration(gen int64) SubscriptionOption {
	return func(s *v1.Subscription) {
		s.Status.ObservedGeneration = gen
	}
}

func WithSubscriptionGenerateName(generateName string) SubscriptionOption {
	return func(c *v1.Subscription) {
		c.ObjectMeta.GenerateName = generateName
	}
}

// WithInitSubscriptionConditions initializes the Subscriptions's conditions.
func WithInitSubscriptionConditions(s *v1.Subscription) {
	s.Status.InitializeConditions()
}

func WithSubscriptionReady(s *v1.Subscription) {
	s.Status = *eventingv1.TestHelper.ReadySubscriptionStatus()
}

// TODO: this can be a runtime object
func WithSubscriptionDeleted(s *v1.Subscription) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	s.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithSubscriptionOwnerReferences(ownerReferences []metav1.OwnerReference) SubscriptionOption {
	return func(c *v1.Subscription) {
		c.ObjectMeta.OwnerReferences = ownerReferences
	}
}

func WithSubscriptionLabels(labels map[string]string) SubscriptionOption {
	return func(c *v1.Subscription) {
		c.ObjectMeta.Labels = labels
	}
}

func WithSubscriptionChannel(gvk metav1.GroupVersionKind, name string) SubscriptionOption {
	return func(s *v1.Subscription) {
		s.Spec.Channel = corev1.ObjectReference{
			APIVersion: apiVersion(gvk),
			Kind:       gvk.Kind,
			Name:       name,
		}
	}
}

func WithSubscriptionSubscriberRef(gvk metav1.GroupVersionKind, name, namespace string) SubscriptionOption {
	return func(s *v1.Subscription) {
		s.Spec.Subscriber = &duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: apiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
				Namespace:  namespace,
			},
		}
	}
}

func WithSubscriptionDeliveryRef(gvk metav1.GroupVersionKind, name, namespace string) SubscriptionOption {
	return func(s *v1.Subscription) {
		s.Spec.Delivery = &eventingduckv1.DeliverySpec{
			DeadLetterSink: &duckv1.Destination{
				Ref: &duckv1.KReference{
					APIVersion: apiVersion(gvk),
					Kind:       gvk.Kind,
					Name:       name,
					Namespace:  namespace,
				},
			},
		}
	}
}

func WithSubscriptionPhysicalSubscriptionSubscriber(uri *apis.URL) SubscriptionOption {
	return func(s *v1.Subscription) {
		if uri == nil {
			panic(errors.New("nil URI"))
		}
		s.Status.PhysicalSubscription.SubscriberURI = uri
	}
}

func WithSubscriptionPhysicalSubscriptionReply(uri *apis.URL) SubscriptionOption {
	return func(s *v1.Subscription) {
		if uri == nil {
			panic(errors.New("nil URI"))
		}
		s.Status.PhysicalSubscription.ReplyURI = uri
	}
}

func WithSubscriptionDeadLetterSinkURI(uri *apis.URL) SubscriptionOption {
	return func(s *v1.Subscription) {
		if uri == nil {
			panic(errors.New("nil URI"))
		}
		s.Status.PhysicalSubscription.DeadLetterSinkURI = uri
	}
}

func WithSubscriptionFinalizers(finalizers ...string) SubscriptionOption {
	return func(s *v1.Subscription) {
		s.Finalizers = finalizers
	}
}

func MarkSubscriptionReady(s *v1.Subscription) {
	s.Status.MarkChannelReady()
	s.Status.MarkReferencesResolved()
	s.Status.MarkAddedToChannel()
}

func MarkAddedToChannel(s *v1.Subscription) {
	s.Status.MarkAddedToChannel()
}

func MarkNotAddedToChannel(reason, msg string) SubscriptionOption {
	return func(s *v1.Subscription) {
		s.Status.MarkNotAddedToChannel(reason, msg)
	}
}

func MarkReferencesResolved(s *v1.Subscription) {
	s.Status.MarkReferencesResolved()
}

func WithSubscriptionReferencesNotResolved(reason, msg string) SubscriptionOption {
	return func(s *v1.Subscription) {
		s.Status.MarkReferencesNotResolved(reason, msg)
	}
}

func WithSubscriptionReferencesResolvedUnknown(reason, msg string) SubscriptionOption {
	return func(s *v1.Subscription) {
		s.Status.MarkReferencesResolvedUnknown(reason, msg)
	}
}

func WithSubscriptionReply(gvk metav1.GroupVersionKind, name, namespace string) SubscriptionOption {
	return func(s *v1.Subscription) {
		s.Spec.Reply = &duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: apiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
				Namespace:  namespace,
			},
		}
	}
}
