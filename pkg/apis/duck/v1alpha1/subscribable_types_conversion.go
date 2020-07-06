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

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	duckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
)

// ConvertTo implements apis.Convertible
// Converts source (from v1alpha1.SubscribableType) into v1beta1.Subscribable
func (source *SubscribableType) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *duckv1beta1.Subscribable:
		sink.ObjectMeta = source.ObjectMeta
		source.Status.ConvertTo(ctx, &sink.Status)
		return source.Spec.ConvertTo(ctx, &sink.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertTo implements apis.Convertible
func (source *SubscribableTypeSpec) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *duckv1beta1.SubscribableSpec:
		if source.Subscribable != nil {
			sink.Subscribers = make([]duckv1beta1.SubscriberSpec, len(source.Subscribable.Subscribers))
			for i, s := range source.Subscribable.Subscribers {
				s.ConvertTo(ctx, &sink.Subscribers[i])
			}
		}
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
	return nil
}

// ConvertTo implements apis.Convertible
func (source *SubscriberSpec) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *duckv1beta1.SubscriberSpec:
		sink.UID = source.UID
		sink.Generation = source.Generation
		sink.SubscriberURI = source.SubscriberURI
		sink.ReplyURI = source.ReplyURI

		if source.Delivery != nil {
			sink.Delivery = source.Delivery
		} else {
			// If however, there's a Deprecated DeadLetterSinkURI, convert that up
			// to DeliverySpec.
			sink.Delivery = &duckv1beta1.DeliverySpec{
				DeadLetterSink: &duckv1.Destination{
					URI: source.DeadLetterSinkURI,
				},
			}
		}
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
	return nil
}

// ConvertTo implements apis.Convertible
func (source *SubscribableTypeStatus) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *duckv1beta1.SubscribableStatus:
		if source.SubscribableStatus != nil &&
			len(source.SubscribableStatus.Subscribers) > 0 {
			sink.Subscribers = make([]duckv1beta1.SubscriberStatus, len(source.SubscribableStatus.Subscribers))
			for i, ss := range source.SubscribableStatus.Subscribers {
				sink.Subscribers[i] = duckv1beta1.SubscriberStatus{
					UID:                ss.UID,
					ObservedGeneration: ss.ObservedGeneration,
					Ready:              ss.Ready,
					Message:            ss.Message,
				}
			}
		}
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
	return nil
}

// ConvertFrom implements apis.Convertible.
// Converts obj v1beta1.Subscribable into v1alpha1.SubscribableType
func (sink *SubscribableType) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *duckv1beta1.Subscribable:
		sink.ObjectMeta = source.ObjectMeta
		sink.Status.ConvertFrom(ctx, &source.Status)
		sink.Spec.ConvertFrom(ctx, &source.Spec)
		return nil
	default:
		return fmt.Errorf("unknown version, got: %T", source)
	}
}

// ConvertFrom implements apis.Convertible
func (sink *SubscribableTypeSpec) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *duckv1beta1.SubscribableSpec:
		if len(source.Subscribers) > 0 {
			sink.Subscribable = &Subscribable{
				Subscribers: make([]SubscriberSpec, len(source.Subscribers)),
			}
			for i, s := range source.Subscribers {
				sink.Subscribable.Subscribers[i].ConvertFrom(ctx, &s)
			}
		}
	default:
		return fmt.Errorf("unknown version, got: %T", source)
	}
	return nil
}

// ConvertFrom implements apis.Convertible
func (sink *SubscriberSpec) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *duckv1beta1.SubscriberSpec:
		var deadLetterSinkURI *apis.URL
		if source.Delivery != nil && source.Delivery.DeadLetterSink != nil {
			deadLetterSinkURI = source.Delivery.DeadLetterSink.URI
		}
		sink.UID = source.UID
		sink.Generation = source.Generation
		sink.SubscriberURI = source.SubscriberURI
		sink.ReplyURI = source.ReplyURI
		sink.Delivery = source.Delivery
		sink.DeadLetterSinkURI = deadLetterSinkURI
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
	return nil
}

// ConvertFrom implements apis.Convertible
func (sink *SubscribableTypeStatus) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *duckv1beta1.SubscribableStatus:
		if len(source.Subscribers) > 0 {
			sink.SubscribableStatus = &SubscribableStatus{
				Subscribers: make([]duckv1beta1.SubscriberStatus, len(source.Subscribers)),
			}
			for i, ss := range source.Subscribers {
				sink.SubscribableStatus.Subscribers[i] = duckv1beta1.SubscriberStatus{
					UID:                ss.UID,
					ObservedGeneration: ss.ObservedGeneration,
					Ready:              ss.Ready,
					Message:            ss.Message,
				}
			}
		}
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
	return nil
}
