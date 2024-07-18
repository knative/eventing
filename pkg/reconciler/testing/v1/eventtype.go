/*
Copyright 2023 The Knative Authors

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

	duckv1 "knative.dev/pkg/apis/duck/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta3"
	"knative.dev/pkg/apis"
)

// EventTypeOption enables further configuration of an EventType.
type EventTypeOption func(*v1beta3.EventType)

// NewEventType creates a EventType with EventTypeOptions.
func NewEventType(name, namespace string, o ...EventTypeOption) *v1beta3.EventType {
	et := &v1beta3.EventType{
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
func WithInitEventTypeConditions(et *v1beta3.EventType) {
	et.Status.InitializeConditions()
}

func WithEventTypeSource(source *apis.URL) EventTypeOption {
	return func(et *v1beta3.EventType) {
		if et.Spec.Attributes == nil {
			et.Spec.Attributes = make([]v1beta3.EventAttributeDefinition, 0)
		}

		et.Spec.Attributes = append(et.Spec.Attributes, v1beta3.EventAttributeDefinition{
			Name:     "source",
			Required: true,
			Value:    source.String(),
		})
	}
}

func WithEventTypeType(t string) EventTypeOption {
	return func(et *v1beta3.EventType) {
		if et.Spec.Attributes == nil {
			et.Spec.Attributes = make([]v1beta3.EventAttributeDefinition, 0)
		}

		et.Spec.Attributes = append(et.Spec.Attributes, v1beta3.EventAttributeDefinition{
			Name:     "type",
			Required: true,
			Value:    t,
		})
	}
}

func WithEventTypeSpecV1() EventTypeOption {
	return func(et *v1beta3.EventType) {
		if et.Spec.Attributes == nil {
			et.Spec.Attributes = make([]v1beta3.EventAttributeDefinition, 0)
		}

		et.Spec.Attributes = append(et.Spec.Attributes, v1beta3.EventAttributeDefinition{
			Name:     "specversion",
			Required: true,
			Value:    "V1",
		})
	}
}

func WithEventTypeEmptyID() EventTypeOption {
	return func(et *v1beta3.EventType) {
		if et.Spec.Attributes == nil {
			et.Spec.Attributes = make([]v1beta3.EventAttributeDefinition, 0)
		}

		et.Spec.Attributes = append(et.Spec.Attributes, v1beta3.EventAttributeDefinition{
			Name:     "id",
			Required: true,
		})
	}
}

func WithEventTypeReference(ref *duckv1.KReference) EventTypeOption {
	return func(et *v1beta3.EventType) {
		et.Spec.Reference = ref
	}
}

func WithEventTypeDescription(description string) EventTypeOption {
	return func(et *v1beta3.EventType) {
		et.Spec.Description = description
	}
}

func WithEventTypeLabels(labels map[string]string) EventTypeOption {
	return func(et *v1beta3.EventType) {
		et.ObjectMeta.Labels = labels
	}
}

func WithEventTypeOwnerReference(ownerRef metav1.OwnerReference) EventTypeOption {
	return func(et *v1beta3.EventType) {
		et.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
			ownerRef,
		}
	}
}

func WithEventTypeDeletionTimestamp(et *v1beta3.EventType) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	et.ObjectMeta.SetDeletionTimestamp(&t)
}

// WithEventTypeResourceDoesNotExist calls .Status.MarkFilterFailed on the EventType.
func WithEventTypeResourceDoesNotExist(et *v1beta3.EventType) {
	et.Status.MarkReferenceDoesNotExist()
}

// WithEventTypeResourceExists calls .Status.MarkReferenceExists on the EventType.
func WithEventTypeResourceExists(et *v1beta3.EventType) {
	et.Status.MarkReferenceExists()
}

func WithEventTypeReferenceNotSet(et *v1beta3.EventType) {
	et.Status.MarkReferenceNotSet()
}
