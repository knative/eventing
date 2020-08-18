/*
Copyright 2020 The Knative Authors
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

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// ChannelOption enables further configuration of a Channel.
type ChannelOption func(*eventingv1.Channel)

// NewChannel creates a Channel with ChannelOptions
func NewChannel(name, namespace string, o ...ChannelOption) *eventingv1.Channel {
	c := &eventingv1.Channel{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "messaging.knative.dev/v1",
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
func WithInitChannelConditions(c *eventingv1.Channel) {
	c.Status.InitializeConditions()
}

func WithNoAnnotations(c *eventingv1.Channel) {
	c.ObjectMeta.Annotations = nil
}

func WithChannelGeneration(gen int64) ChannelOption {
	return func(s *eventingv1.Channel) {
		s.Generation = gen
	}
}

func WithChannelObservedGeneration(gen int64) ChannelOption {
	return func(s *eventingv1.Channel) {
		s.Status.ObservedGeneration = gen
	}
}

func WithChannelDeleted(c *eventingv1.Channel) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithChannelTemplate(typeMeta metav1.TypeMeta) ChannelOption {
	return func(c *eventingv1.Channel) {
		c.Spec.ChannelTemplate = &messagingv1.ChannelTemplateSpec{
			TypeMeta: typeMeta,
		}
	}
}

func WithBackingChannelFailed(reason, msg string) ChannelOption {
	return func(c *eventingv1.Channel) {
		c.Status.MarkBackingChannelFailed(reason, msg)
	}
}

func WithBackingChannelUnknown(reason, msg string) ChannelOption {
	return func(c *eventingv1.Channel) {
		c.Status.MarkBackingChannelUnknown(reason, msg)
	}
}

func WithBackingChannelReady(c *eventingv1.Channel) {
	c.Status.MarkBackingChannelReady()
}

func WithBackingChannelObjRef(objRef *duckv1.KReference) ChannelOption {
	return func(c *eventingv1.Channel) {
		c.Status.Channel = objRef
	}
}

func WithChannelNoAddress() ChannelOption {
	return func(c *eventingv1.Channel) {
		c.Status.SetAddress(nil)
	}
}

func WithChannelAddress(hostname string) ChannelOption {
	return func(c *eventingv1.Channel) {
		c.Status.SetAddress(&duckv1.Addressable{URL: apis.HTTP(hostname)})
	}
}

func WithChannelReadySubscriber(uid string) ChannelOption {
	return WithChannelReadySubscriberAndGeneration(uid, 0)
}

func WithChannelReadySubscriberAndGeneration(uid string, observedGeneration int64) ChannelOption {
	return func(c *eventingv1.Channel) {
		c.Status.Subscribers = append(c.Status.Subscribers, eventingduckv1.SubscriberStatus{
			UID:                types.UID(uid),
			ObservedGeneration: observedGeneration,
			Ready:              v1.ConditionTrue,
		})
	}
}

func WithChannelSubscriberStatuses(subscriberStatuses []eventingduckv1.SubscriberStatus) ChannelOption {
	return func(c *eventingv1.Channel) {
		c.Status.Subscribers = subscriberStatuses
	}
}
