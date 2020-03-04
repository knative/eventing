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

package v1alpha1

import (
	"context"
	"fmt"

	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/pkg/apis"
)

// ConvertTo implements apis.Convertible.
// Converts source (from v1alpha1.Trigger) into v1beta1.Trigger
func (source *Trigger) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta1.Trigger:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.Broker = source.Spec.Broker
		sink.Spec.Subscriber = source.Spec.Subscriber
		if source.Spec.Filter != nil {
			sink.Spec.Filter = &v1beta1.TriggerFilter{
			}
			if source.Spec.Filter.Attributes != nil {
				sink.Spec.Filter = &v1beta1.TriggerFilter{
					Attributes: make(v1beta1.TriggerFilterAttributes, len(*source.Spec.Filter.Attributes)),
				}
				for k, v := range *source.Spec.Filter.Attributes {
					sink.Spec.Filter.Attributes[k] = v
				}
			}
			if source.Spec.Filter.DeprecatedSourceAndType != nil {
				sink.Spec.Filter = &v1beta1.TriggerFilter{
					Attributes: make(v1beta1.TriggerFilterAttributes, 2),
				}
				sink.Spec.Filter.Attributes["source"] = source.Spec.Filter.DeprecatedSourceAndType.Source
				sink.Spec.Filter.Attributes["type"] = source.Spec.Filter.DeprecatedSourceAndType.Type
			}
		}
		sink.Status.Status = source.Status.Status
		sink.Status.SubscriberURI = source.Status.SubscriberURI
		return nil
	default:
		return fmt.Errorf("Unknown conversion, got: %T", sink)

	}
}

// ConvertFrom implements apis.Convertible.
// Converts obj from v1beta1.Trigger into v1alpha1.Trigger
func (sink *Trigger) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta1.Trigger:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.Broker = source.Spec.Broker
		sink.Spec.Subscriber = source.Spec.Subscriber
		if source.Spec.Filter != nil && source.Spec.Filter.Attributes != nil {
			attributes := TriggerFilterAttributes{}
			for k, v := range source.Spec.Filter.Attributes {
				attributes[k] = v
			}
			sink.Spec.Filter = &TriggerFilter{
				Attributes: &attributes,
			}
		}

		sink.Status.Status = source.Status.Status
		sink.Status.SubscriberURI = source.Status.SubscriberURI
		return nil
	default:
		return fmt.Errorf("Unknown conversion, got: %T", source)
	}
}
