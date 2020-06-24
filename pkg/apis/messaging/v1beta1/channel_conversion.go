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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	duckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1beta1"
	v1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgduckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// ConvertTo implements apis.Convertible
// Converts source (from v1beta1.Channel) into v1.Channel
func (source *Channel) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1.Channel:
		sink.ObjectMeta = source.ObjectMeta
		source.Status.ConvertTo(ctx, &sink.Status)
		return source.Spec.ConvertTo(ctx, &sink.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertTo helps implement apis.Convertible
func (source *ChannelSpec) ConvertTo(ctx context.Context, sink *v1.ChannelSpec) error {
	sink.ChannelTemplate = &v1.ChannelTemplateSpec{
		TypeMeta: source.ChannelTemplate.DeepCopy().TypeMeta,
		Spec:     source.ChannelTemplate.DeepCopy().Spec,
	}
	sink.ChannelableSpec = eventingduckv1.ChannelableSpec{}
	source.Delivery.ConvertTo(ctx, sink.Delivery)
	source.ChannelableSpec.SubscribableSpec.ConvertTo(ctx, &sink.ChannelableSpec.SubscribableSpec)

	return nil
}

// ConvertTo helps implement apis.Convertible
func (source *ChannelStatus) ConvertTo(ctx context.Context, sink *v1.ChannelStatus) {
	source.Status.ConvertTo(ctx, &sink.Status)
	if source.AddressStatus.Address != nil {
		sink.AddressStatus.Address = &duckv1.Addressable{}
		source.AddressStatus.Address.ConvertTo(ctx, sink.AddressStatus.Address)
	}
	if source.SubscribableTypeStatus.SubscribableStatus != nil &&
		len(source.SubscribableTypeStatus.SubscribableStatus.Subscribers) > 0 {
		sink.SubscribableStatus.Subscribers = make([]eventingduckv1.SubscriberStatus, len(source.SubscribableTypeStatus.SubscribableStatus.Subscribers))
		for i, ss := range source.SubscribableTypeStatus.SubscribableStatus.Subscribers {
			sink.SubscribableStatus.Subscribers[i] = eventingduckv1.SubscriberStatus{
				UID:                ss.UID,
				ObservedGeneration: ss.ObservedGeneration,
				Ready:              ss.Ready,
				Message:            ss.Message,
			}
		}
	}
	if source.Channel != nil {
		sink.Channel = &duckv1.KReference{
			Kind:       source.Channel.Kind,
			APIVersion: source.Channel.APIVersion,
			Name:       source.Channel.Name,
			Namespace:  source.Channel.Namespace,
		}
	}
}

// ConvertFrom implements apis.Convertible.
// Converts obj v1.Channel into v1beta1.Channel
func (sink *Channel) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1.Channel:
		sink.ObjectMeta = source.ObjectMeta
		sink.Status.ConvertFrom(ctx, source.Status)
		sink.Spec.ConvertFrom(ctx, source.Spec)
		return nil
	default:
		return fmt.Errorf("unknown version, got: %T", source)
	}
}

// ConvertFrom helps implement apis.Convertible
func (sink *ChannelSpec) ConvertFrom(ctx context.Context, source v1.ChannelSpec) {
	sink.ChannelTemplate = source.ChannelTemplate
	sink.Delivery = source.ChannelableSpec.Delivery
	if len(source.ChannelableSpec.Subscribers) > 0 {
		sink.Subscribable = &eventingduck.Subscribable{
			Subscribers: make([]eventingduck.SubscriberSpec, len(source.ChannelableSpec.Subscribers)),
		}
		for i, s := range source.ChannelableSpec.Subscribers {
			sink.Subscribable.Subscribers[i] = eventingduck.SubscriberSpec{
				UID:           s.UID,
				Generation:    s.Generation,
				SubscriberURI: s.SubscriberURI,
				ReplyURI:      s.ReplyURI,
				Delivery:      s.Delivery,
			}
		}
	}
}

// ConvertFrom helps implement apis.Convertible
func (sink *ChannelStatus) ConvertFrom(ctx context.Context, source v1.ChannelStatus) error {
	source.Status.ConvertTo(ctx, &sink.Status)
	if source.AddressStatus.Address != nil {
		sink.AddressStatus.Address = &pkgduckv1beta1.Addressable{}
		if err := sink.AddressStatus.Address.ConvertFrom(ctx, source.AddressStatus.Address); err != nil {
			return err
		}
	}
	if len(source.SubscribableStatus.Subscribers) > 0 {
		sink.SubscribableTypeStatus.SubscribableStatus = &duckv1beta1.SubscribableStatus{
			Subscribers: make([]eventingduckv1.SubscriberStatus, len(source.SubscribableStatus.Subscribers)),
		}
		copy(sink.SubscribableTypeStatus.SubscribableStatus.Subscribers, source.SubscribableStatus.Subscribers)
	}
	if source.Channel != nil {
		sink.Channel = &corev1.ObjectReference{
			Kind:       source.Channel.Kind,
			APIVersion: source.Channel.APIVersion,
			Name:       source.Channel.Name,
			Namespace:  source.Channel.Namespace,
		}
	}
	return nil
}
