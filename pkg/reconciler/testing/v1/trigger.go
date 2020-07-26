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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// TriggerOption enables further configuration of a Trigger.
type TriggerOption func(*v1.Trigger)

// NewTrigger creates a Trigger with TriggerOptions.
func NewTrigger(name, namespace, broker string, to ...TriggerOption) *v1.Trigger {
	t := &v1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.TriggerSpec{
			Broker: broker,
		},
	}
	for _, opt := range to {
		opt(t)
	}
	t.SetDefaults(context.Background())
	return t
}

func WithTriggerSubscriberURI(rawurl string) TriggerOption {
	uri, _ := apis.ParseURL(rawurl)
	return func(t *v1.Trigger) {
		t.Spec.Subscriber = duckv1.Destination{URI: uri}
	}
}

func WithTriggerSubscriberRef(gvk metav1.GroupVersionKind, name, namespace string) TriggerOption {
	return func(t *v1.Trigger) {
		t.Spec.Subscriber = duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: apiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
				Namespace:  namespace,
			},
		}
	}
}

func WithTriggerSubscriberRefAndURIReference(gvk metav1.GroupVersionKind, name, namespace string, rawuri string) TriggerOption {
	uri, _ := apis.ParseURL(rawuri)
	return func(t *v1.Trigger) {
		t.Spec.Subscriber = duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: apiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
				Namespace:  namespace,
			},
			URI: uri,
		}
	}
}

// WithInitTriggerConditions initializes the Triggers's conditions.
func WithInitTriggerConditions(t *v1.Trigger) {
	t.Status.InitializeConditions()
}

func WithTriggerGeneration(gen int64) TriggerOption {
	return func(s *v1.Trigger) {
		s.Generation = gen
	}
}

func WithTriggerStatusObservedGeneration(gen int64) TriggerOption {
	return func(s *v1.Trigger) {
		s.Status.ObservedGeneration = gen
	}
}

// WithTriggerBrokerReady initializes the Triggers's conditions.
func WithTriggerBrokerReady() TriggerOption {
	return func(t *v1.Trigger) {
		t.Status.PropagateBrokerCondition(v1.TestHelper.ReadyBrokerCondition())
	}
}

// WithTriggerBrokerFailed marks the Broker as failed
func WithTriggerBrokerFailed(reason, message string) TriggerOption {
	return func(t *v1.Trigger) {
		t.Status.MarkBrokerFailed(reason, message)
	}
}

// WithTriggerBrokerNotConfigured marks the Broker as not having been reconciled.
func WithTriggerBrokerNotConfigured() TriggerOption {
	return func(t *v1.Trigger) {
		t.Status.MarkBrokerNotConfigured()
	}
}

// WithTriggerBrokerUnknown marks the Broker as unknown
func WithTriggerBrokerUnknown(reason, message string) TriggerOption {
	return func(t *v1.Trigger) {
		t.Status.MarkBrokerUnknown(reason, message)
	}
}

func WithTriggerNotSubscribed(reason, message string) TriggerOption {
	return func(t *v1.Trigger) {
		t.Status.MarkNotSubscribed(reason, message)
	}
}

func WithTriggerSubscribedUnknown(reason, message string) TriggerOption {
	return func(t *v1.Trigger) {
		t.Status.MarkSubscribedUnknown(reason, message)
	}
}

func WithTriggerSubscriptionNotConfigured() TriggerOption {
	return func(t *v1.Trigger) {
		t.Status.MarkSubscriptionNotConfigured()
	}
}

func WithTriggerSubscribed() TriggerOption {
	return func(t *v1.Trigger) {
		t.Status.PropagateSubscriptionCondition(v1.TestHelper.ReadySubscriptionCondition())
	}
}

func WithTriggerStatusSubscriberURI(uri string) TriggerOption {
	return func(t *v1.Trigger) {
		u, _ := apis.ParseURL(uri)
		t.Status.SubscriberURI = u
	}
}

func WithAnnotation(key, value string) TriggerOption {
	return func(t *v1.Trigger) {
		if t.Annotations == nil {
			t.Annotations = make(map[string]string)
		}
		t.Annotations[key] = value
	}
}

func WithDependencyAnnotation(dependencyAnnotation string) TriggerOption {
	return func(t *v1.Trigger) {
		if t.Annotations == nil {
			t.Annotations = make(map[string]string)
		}
		t.Annotations[v1.DependencyAnnotation] = dependencyAnnotation
	}
}

func WithTriggerDependencyReady() TriggerOption {
	return func(t *v1.Trigger) {
		t.Status.MarkDependencySucceeded()
	}
}

func WithTriggerDependencyFailed(reason, message string) TriggerOption {
	return func(t *v1.Trigger) {
		t.Status.MarkDependencyFailed(reason, message)
	}
}

func WithTriggerDependencyUnknown(reason, message string) TriggerOption {
	return func(t *v1.Trigger) {
		t.Status.MarkDependencyUnknown(reason, message)
	}
}

func WithTriggerSubscriberResolvedSucceeded() TriggerOption {
	return func(t *v1.Trigger) {
		t.Status.MarkSubscriberResolvedSucceeded()
	}
}

func WithTriggerSubscriberResolvedFailed(reason, message string) TriggerOption {
	return func(t *v1.Trigger) {
		t.Status.MarkSubscriberResolvedFailed(reason, message)
	}
}

func WithTriggerSubscriberResolvedUnknown(reason, message string) TriggerOption {
	return func(t *v1.Trigger) {
		t.Status.MarkSubscriberResolvedUnknown(reason, message)
	}
}

func WithTriggerDeleted(t *v1.Trigger) {
	deleteTime := metav1.NewTime(time.Unix(1e9, 0))
	t.ObjectMeta.SetDeletionTimestamp(&deleteTime)
}

func WithTriggerUID(uid string) TriggerOption {
	return func(t *v1.Trigger) {
		t.UID = types.UID(uid)
	}
}
