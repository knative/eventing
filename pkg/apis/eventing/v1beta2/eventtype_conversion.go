/*
Copyright 2021 The Knative Authors

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

package v1beta2

import (
	"context"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	eventing "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta3"
)

// ConvertTo converts the receiver into `to`.
func (source *EventType) ConvertTo(ctx context.Context, to apis.Convertible) error {
	switch sink := to.(type) {
	case *v1beta3.EventType:

		source.ObjectMeta.DeepCopyInto(&sink.ObjectMeta)
		source.Status.Status.DeepCopyInto(&sink.Status.Status)

		sink.Spec.Reference = source.Spec.Reference.DeepCopy()
		sink.Spec.Description = source.Spec.Description

		if source.Spec.Reference == nil && source.Spec.Broker != "" {
			source.Spec.Reference = &duckv1.KReference{
				Kind:       "Broker",
				Name:       source.Spec.Broker,
				APIVersion: eventing.SchemeGroupVersion.String(),
			}
		}

		sink.Spec.Attributes = []v1beta3.EventAttributeDefinition{}
		if source.Spec.Type != "" {
			sink.Spec.Attributes = append(sink.Spec.Attributes, v1beta3.EventAttributeDefinition{
				Name:     "type",
				Required: true,
				Value:    source.Spec.Type,
			})
		}
		if source.Spec.Schema != nil {
			sink.Spec.Attributes = append(sink.Spec.Attributes, v1beta3.EventAttributeDefinition{
				Name:     "schemadata",
				Required: false,
				Value:    source.Spec.Schema.String(),
			})
		}
		if source.Spec.Source != nil {
			sink.Spec.Attributes = append(sink.Spec.Attributes, v1beta3.EventAttributeDefinition{
				Name:     "source",
				Required: true,
				Value:    source.Spec.Source.String(),
			})
		}
		return nil
	default:
		return apis.ConvertToViaProxy(ctx, source, &v1beta3.EventType{}, to)
	}

}

// ConvertFrom implements apis.Convertible
func (sink *EventType) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	switch source := from.(type) {
	case *v1beta3.EventType:

		source.ObjectMeta.DeepCopyInto(&sink.ObjectMeta)
		source.Status.Status.DeepCopyInto(&sink.Status.Status)

		sink.Spec.Reference = source.Spec.Reference.DeepCopy()
		sink.Spec.Description = source.Spec.Description

		for _, at := range source.Spec.Attributes {
			switch at.Name {
			case "source":
				sink.Spec.Source, _ = apis.ParseURL(at.Value)
			case "type":
				sink.Spec.Type = at.Value
			case "schemadata":
				sink.Spec.Schema, _ = apis.ParseURL(at.Value)
			}
		}

		return nil
	default:
		return apis.ConvertFromViaProxy(ctx, from, &v1beta3.EventType{}, sink)
	}
}
