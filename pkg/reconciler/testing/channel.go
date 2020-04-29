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

	"k8s.io/apimachinery/pkg/types"

	v1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/apis/messaging/v1beta1"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// ChannelOption enables further configuration of a Channel.
type ChannelOptionV1Beta1 func(*v1beta1.Channel)

// NewChannel creates a Channel with ChannelOptions
func NewChannelV1Beta1(name, namespace string, o ...ChannelOptionV1Beta1) *v1beta1.Channel {
	c := &v1beta1.Channel{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "messaging.knative.dev/v1beta1",
			Kind:       "Channel",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range o {
		opt(c)
	}
	c.SetDefaults(context.Background())
	return c
}

// WithInitChannelConditions initializes the Channel's conditions.
func WithInitChannelConditionsV1Beta1(c *v1beta1.Channel) {
	c.Status.InitializeConditions()
}

func WithChannelGenerationV1Beta1(gen int64) ChannelOptionV1Beta1 {
	return func(s *v1beta1.Channel) {
		s.Generation = gen
	}
}
func WithChannelObservedGenerationV1Beta1(gen int64) ChannelOptionV1Beta1 {
	return func(s *v1beta1.Channel) {
		s.Status.ObservedGeneration = gen
	}
}

func WithChannelDeletedV1Beta1(c *v1beta1.Channel) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithChannelTemplateV1Beta1(typeMeta metav1.TypeMeta) ChannelOptionV1Beta1 {
	return func(c *v1beta1.Channel) {
		c.Spec.ChannelTemplate = &messagingv1beta1.ChannelTemplateSpec{
			TypeMeta: typeMeta,
		}
	}
}

func WithBackingChannelFailedV1Beta1(reason, msg string) ChannelOptionV1Beta1 {
	return func(c *v1beta1.Channel) {
		c.Status.MarkBackingChannelFailed(reason, msg)
	}
}

func WithBackingChannelUnknownV1Beta1(reason, msg string) ChannelOptionV1Beta1 {
	return func(c *v1beta1.Channel) {
		c.Status.MarkBackingChannelUnknown(reason, msg)
	}
}

func WithBackingChannelReadyV1Beta1(c *v1beta1.Channel) {
	c.Status.MarkBackingChannelReady()
}

func WithBackingChannelObjRefV1Beta1(objRef *duckv1.KReference) ChannelOptionV1Beta1 {
	return func(c *v1beta1.Channel) {
		c.Status.Channel = objRef
	}
}

func WithChannelNoAddressV1Beta1() ChannelOptionV1Beta1 {
	return func(c *v1beta1.Channel) {
		c.Status.SetAddress(nil)
	}
}

func WithChannelAddressV1Beta1(hostname string) ChannelOptionV1Beta1 {
	return func(c *v1beta1.Channel) {
		c.Status.SetAddress(&duckv1.Addressable{URL: apis.HTTP(hostname)})
	}
}

func WithChannelReadySubscriberV1Beta1(uid string) ChannelOptionV1Beta1 {
	return WithChannelReadySubscriberAndGenerationV1Beta1(uid, 0)
}

func WithChannelReadySubscriberAndGenerationV1Beta1(uid string, observedGeneration int64) ChannelOptionV1Beta1 {
	return func(c *v1beta1.Channel) {
		c.Status.Subscribers = append(c.Status.Subscribers, eventingduckv1beta1.SubscriberStatus{
			UID:                types.UID(uid),
			ObservedGeneration: observedGeneration,
			Ready:              v1.ConditionTrue,
		})
	}
}

func WithChannelSubscriberStatusesV1Beta1(subscriberStatuses []eventingduckv1beta1.SubscriberStatus) ChannelOptionV1Beta1 {
	return func(c *v1beta1.Channel) {
		c.Status.Subscribers = subscriberStatuses
	}
}

// ChannelOption enables further configuration of a Channel.
type ChannelOption func(*v1alpha1.Channel)

// NewChannel creates a Channel with ChannelOptions
func NewChannel(name, namespace string, o ...ChannelOption) *v1alpha1.Channel {
	c := &v1alpha1.Channel{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "messaging.knative.dev/v1alpha1",
			Kind:       "Channel",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range o {
		opt(c)
	}
	c.SetDefaults(context.Background())
	return c
}

// WithInitChannelConditions initializes the Channel's conditions.
func WithInitChannelConditions(c *v1alpha1.Channel) {
	c.Status.InitializeConditions()
}

func WithChannelGeneration(gen int64) ChannelOption {
	return func(s *v1alpha1.Channel) {
		s.Generation = gen
	}
}
func WithChannelObservedGeneration(gen int64) ChannelOption {
	return func(s *v1alpha1.Channel) {
		s.Status.ObservedGeneration = gen
	}
}

func WithChannelDeleted(c *v1alpha1.Channel) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithChannelTemplate(typeMeta metav1.TypeMeta) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Spec.ChannelTemplate = &messagingv1beta1.ChannelTemplateSpec{
			TypeMeta: typeMeta,
		}
	}
}

func WithBackingChannelFailed(reason, msg string) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Status.MarkBackingChannelFailed(reason, msg)
	}
}

func WithBackingChannelUnknown(reason, msg string) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Status.MarkBackingChannelUnknown(reason, msg)
	}
}

func WithBackingChannelReady(c *v1alpha1.Channel) {
	c.Status.MarkBackingChannelReady()
}

func WithBackingChannelObjRef(objRef *v1.ObjectReference) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Status.Channel = objRef
	}
}

func WithChannelNoAddress() ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Status.SetAddress(nil)
	}
}

func WithChannelAddress(hostname string) ChannelOption {
	return func(c *v1alpha1.Channel) {
		address := &duckv1alpha1.Addressable{
			Addressable: duckv1beta1.Addressable{
				URL: &apis.URL{
					Scheme: "http",
					Host:   hostname,
				},
			},
		}
		c.Status.SetAddress(address)
	}
}

func WithChannelReadySubscriber(uid string) ChannelOption {
	return WithChannelReadySubscriberAndGeneration(uid, 0)
}

func WithChannelReadySubscriberAndGeneration(uid string, observedGeneration int64) ChannelOption {
	return func(c *v1alpha1.Channel) {
		if c.Status.GetSubscribableTypeStatus() == nil { // Both the SubscribableStatus fields are nil
			c.Status.SetSubscribableTypeStatus(eventingduckv1alpha1.SubscribableStatus{})
		}
		c.Status.SubscribableTypeStatus.AddSubscriberToSubscribableStatus(eventingduckv1alpha1.SubscriberStatus{
			UID:                types.UID(uid),
			ObservedGeneration: observedGeneration,
			Ready:              v1.ConditionTrue,
		})
	}
}

func WithChannelSubscriberStatuses(subscriberStatuses []eventingduckv1alpha1.SubscriberStatus) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Status.SetSubscribableTypeStatus(eventingduckv1alpha1.SubscribableStatus{
			Subscribers: subscriberStatuses,
		})
	}
}
