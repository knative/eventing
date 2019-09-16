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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
)

const (
	unmarshalFailedDependencyAnnotation = "{" +
		"\"kind\":{CronJobSource}, " +
		"\"name\":\"test-cronjob-source\"," +
		"\"apiVersion\":\"sources.eventing.knative.dev/v1alpha1\"" +
		"}"
)

// TriggerOption enables further configuration of a Trigger.
type TriggerOption func(*v1alpha1.Trigger)

// NewTrigger creates a Trigger with TriggerOptions.
func NewTrigger(name, namespace, broker string, to ...TriggerOption) *v1alpha1.Trigger {
	t := &v1alpha1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.TriggerSpec{
			Broker: broker,
		},
	}
	for _, opt := range to {
		opt(t)
	}
	t.SetDefaults(context.Background())
	return t
}

func WithTriggerSubscriberURI(uri string) TriggerOption {
	return func(t *v1alpha1.Trigger) {
		t.Spec.Subscriber = &messagingv1alpha1.SubscriberSpec{URI: &uri}
	}
}

func WithTriggerSubscriberRef(gvk metav1.GroupVersionKind, name string) TriggerOption {
	return func(t *v1alpha1.Trigger) {
		t.Spec.Subscriber = &messagingv1alpha1.SubscriberSpec{
			Ref: &corev1.ObjectReference{
				APIVersion: apiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

// WithInitTriggerConditions initializes the Triggers's conditions.
func WithInitTriggerConditions(t *v1alpha1.Trigger) {
	t.Status.InitializeConditions()
}

// WithTriggerBrokerReady initializes the Triggers's conditions.
func WithTriggerBrokerReady() TriggerOption {
	return func(t *v1alpha1.Trigger) {
		t.Status.PropagateBrokerStatus(v1alpha1.TestHelper.ReadyBrokerStatus())
	}
}

// WithTriggerBrokerFailed marks the Broker as failed
func WithTriggerBrokerFailed(reason, message string) TriggerOption {
	return func(t *v1alpha1.Trigger) {
		t.Status.MarkBrokerFailed(reason, message)
	}
}

func WithTriggerNotSubscribed(reason, message string) TriggerOption {
	return func(t *v1alpha1.Trigger) {
		t.Status.MarkNotSubscribed(reason, message)
	}
}

func WithTriggerSubscribed() TriggerOption {
	return func(t *v1alpha1.Trigger) {
		t.Status.PropagateSubscriptionStatus(v1alpha1.TestHelper.ReadySubscriptionStatus())
	}
}

func WithTriggerStatusSubscriberURI(uri string) TriggerOption {
	return func(t *v1alpha1.Trigger) {
		t.Status.SubscriberURI = uri
	}
}

func WithUnmarshalFailedDependencyAnnotation() TriggerOption {
	return func(t *v1alpha1.Trigger) {
		if t.Annotations == nil {
			t.Annotations = make(map[string]string)
		}
		t.Annotations[v1alpha1.DependencyAnnotation] = unmarshalFailedDependencyAnnotation
	}
}

func WithDependencyAnnotation(dependencyAnnotation string) TriggerOption {
	return func(t *v1alpha1.Trigger) {
		if t.Annotations == nil {
			t.Annotations = make(map[string]string)
		}
		t.Annotations[v1alpha1.DependencyAnnotation] = dependencyAnnotation
	}
}

func WithTriggerDependencyReady() TriggerOption {
	return func(t *v1alpha1.Trigger) {
		t.Status.MarkDependencySucceeded()
	}
}

func WithTriggerDependencyFailed(reason, message string) TriggerOption {
	return func(t *v1alpha1.Trigger) {
		t.Status.MarkDependencyFailed(reason, message)
	}
}

func WithTriggerDependencyUnknown(reason, message string) TriggerOption {
	return func(t *v1alpha1.Trigger) {
		t.Status.MarkDependencyUnknown(reason, message)
	}
}

// TODO: this can be a runtime object
func WithTriggerDeleted(t *v1alpha1.Trigger) {
	deleteTime := metav1.NewTime(time.Unix(1e9, 0))
	t.ObjectMeta.SetDeletionTimestamp(&deleteTime)
}

func WithTriggerUID(uid string) TriggerOption {
	return func(t *v1alpha1.Trigger) {
		t.UID = types.UID(uid)
	}
}
