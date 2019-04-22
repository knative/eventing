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

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TriggerOption enables further configuration of a Trigger.
type TriggerOption func(*v1alpha1.Trigger)

// NewTrigger creates a Trigger with TriggerOptions.
func NewTrigger(name, namespace string, so ...TriggerOption) *v1alpha1.Trigger {
	t := &v1alpha1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range so {
		opt(t)
	}
	t.SetDefaults(context.Background())
	return t
}

func WithTriggerSubscriberURI(uri string) TriggerOption {
	return func(t *v1alpha1.Trigger) {
		t.Spec.Subscriber = &v1alpha1.SubscriberSpec{URI: &uri}
	}
}

func WithTriggerSubscriberRef(gvk metav1.GroupVersionKind, name string) TriggerOption {
	return func(t *v1alpha1.Trigger) {
		t.Spec.Subscriber = &v1alpha1.SubscriberSpec{
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

// WithBrokerNotReady initializes the Triggers's conditions.
func WithTriggerBrokerFailed(reason, message string) TriggerOption {
	return func(t *v1alpha1.Trigger) {
		t.Status.MarkBrokerFailed(reason, message)
	}
}

// TODO: this can be a runtime object
func WithTriggerDeleted(t *v1alpha1.Trigger) {
	deleteTime := metav1.NewTime(time.Unix(1e9, 0))
	t.ObjectMeta.SetDeletionTimestamp(&deleteTime)
}
