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

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/pkg/apis"
)

// ConvertUp implements apis.Convertible.
// Converts source (from v1alpha1.Subscription) into v1beta1.Subscription
func (source *Subscription) ConvertUp(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta1.Subscription:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.Channel = source.Spec.Channel
		if source.Spec.Delivery != nil {
			sink.Spec.Delivery = &duckv1beta1.DeliverySpec{}
			if err := source.Spec.Delivery.ConvertUp(ctx, sink.Spec.Delivery); err != nil {
				return err
			}
		}
		sink.Spec.Subscriber = source.Spec.Subscriber
		sink.Spec.Reply = source.Spec.Reply

		sink.Status.Status = source.Status.Status
		sink.Status.PhysicalSubscription.SubscriberURI = source.Status.PhysicalSubscription.SubscriberURI
		sink.Status.PhysicalSubscription.ReplyURI = source.Status.PhysicalSubscription.ReplyURI
		sink.Status.PhysicalSubscription.DeadLetterSinkURI = source.Status.PhysicalSubscription.DeadLetterSinkURI
		return nil
	default:
		return fmt.Errorf("Unknown conversion, got: %T", sink)

	}
}

// ConvertDown implements apis.Convertible.
// Converts obj from v1beta1.Subscription into v1alpha1.Subscription
func (sink *Subscription) ConvertDown(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta1.Subscription:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.Channel = source.Spec.Channel
		if source.Spec.Delivery != nil {
			sink.Spec.Delivery = &duckv1alpha1.DeliverySpec{}
			if err := sink.Spec.Delivery.ConvertDown(ctx, source.Spec.Delivery); err != nil {
				return err
			}
		}
		sink.Spec.Subscriber = source.Spec.Subscriber
		sink.Spec.Reply = source.Spec.Reply

		sink.Status.Status = source.Status.Status
		sink.Status.PhysicalSubscription.SubscriberURI = source.Status.PhysicalSubscription.SubscriberURI
		sink.Status.PhysicalSubscription.ReplyURI = source.Status.PhysicalSubscription.ReplyURI
		sink.Status.PhysicalSubscription.DeadLetterSinkURI = source.Status.PhysicalSubscription.DeadLetterSinkURI

		return nil
	default:
		return fmt.Errorf("Unknown conversion, got: %T", source)
	}
}
