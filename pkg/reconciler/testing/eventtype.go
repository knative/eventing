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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/pkg/apis"
)

// EventTypeOption enables further configuration of an EventType.
type EventTypeOption func(*v1beta1.EventType)

// NewEventType creates a EventType with EventTypeOptions.
func NewEventType(name, namespace string, o ...EventTypeOption) *v1beta1.EventType {
	et := &v1beta1.EventType{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	for _, opt := range o {
		opt(et)
	}
	et.SetDefaults(context.Background())
	return et
}

// WithInitEventTypeConditions initializes the EventType's conditions.
func WithInitEventTypeConditions(et *v1beta1.EventType) {
	et.Status.InitializeConditions()
}

func WithEventTypeSource(source *apis.URL) EventTypeOption {
	return func(et *v1beta1.EventType) {
		et.Spec.Source = source
	}
}

func WithEventTypeType(t string) EventTypeOption {
	return func(et *v1beta1.EventType) {
		et.Spec.Type = t
	}
}

func WithEventTypeBroker(broker string) EventTypeOption {
	return func(et *v1beta1.EventType) {
		et.Spec.Broker = broker
	}
}

func WithEventTypeDescription(description string) EventTypeOption {
	return func(et *v1beta1.EventType) {
		et.Spec.Description = description
	}
}

func WithEventTypeLabels(labels map[string]string) EventTypeOption {
	return func(et *v1beta1.EventType) {
		et.ObjectMeta.Labels = labels
	}
}

func WithEventTypeOwnerReference(ownerRef metav1.OwnerReference) EventTypeOption {
	return func(et *v1beta1.EventType) {
		et.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
			ownerRef,
		}
	}
}

func WithEventTypeDeletionTimestamp(et *v1beta1.EventType) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	et.ObjectMeta.SetDeletionTimestamp(&t)
}

// WithEventTypeBrokerNotFound calls .Status.MarkFilterFailed on the EventType.
func WithEventTypeBrokerDoesNotExist(et *v1beta1.EventType) {
	et.Status.MarkBrokerDoesNotExist()
}

// WithEventTypeBrokerExists calls .Status.MarkBrokerExists on the EventType.
func WithEventTypeBrokerExists(et *v1beta1.EventType) {
	et.Status.MarkBrokerExists()
}

func WithEventTypeBrokerFailed(reason, message string) EventTypeOption {
	return func(et *v1beta1.EventType) {
		et.Status.MarkBrokerFailed(reason, message)
	}
}

func WithEventTypeBrokerUnknown(reason, message string) EventTypeOption {
	return func(et *v1beta1.EventType) {
		et.Status.MarkBrokerUnknown(reason, message)
	}
}

// WithEventTypeBrokerReady calls .Status.MarkBrokerReady on the EventType.
func WithEventTypeBrokerReady(et *v1beta1.EventType) {
	et.Status.MarkBrokerReady()
}
