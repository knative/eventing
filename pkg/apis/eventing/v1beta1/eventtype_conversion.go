/*
Copyright 2020 The Knative Authors.

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

package v1beta1

import (
	"context"

	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/eventing/pkg/apis/eventing/v1beta2"

	"knative.dev/pkg/apis"
)

// ConvertTo implements apis.Convertible
func (source *EventType) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta2.EventType:
		sink.ObjectMeta = source.ObjectMeta
		sink.Status = v1beta2.EventTypeStatus{
			Status: source.Status.Status,
		}
		sink.Spec = v1beta2.EventTypeSpec{
			Type:       source.Spec.Type,
			Source:     source.Spec.Source,
			Schema:     source.Spec.Schema,
			SchemaData: source.Spec.SchemaData,
			Reference: &duckv1.KReference{
				APIVersion: "eventing.knative.dev/v1",
				Kind:       "Broker",
				Name:       source.Spec.Broker,
			},
			Description: source.Spec.Description,
		}

		return nil
	default:
		return apis.ConvertToViaProxy(ctx, source, &v1beta2.EventType{}, sink)
	}
}

// ConvertFrom implements apis.Convertible
func (sink *EventType) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta2.EventType:
		sink.ObjectMeta = source.ObjectMeta
		sink.Status = EventTypeStatus{
			Status: source.Status.Status,
		}

		sink.Spec = EventTypeSpec{
			Type:        source.Spec.Type,
			Source:      source.Spec.Source,
			Schema:      source.Spec.Schema,
			SchemaData:  source.Spec.SchemaData,
			Broker:      source.Spec.Reference.Name,
			Description: source.Spec.Description,
		}

		return nil
	default:
		return apis.ConvertFromViaProxy(ctx, source, &v1beta2.EventType{}, sink)
	}
}
