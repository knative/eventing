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

	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/pkg/apis/messaging/v1beta1"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// ChannelOption enables further configuration of a Channel.
type ChannelOption func(*v1beta1.Channel)

// NewChannel creates a Channel with ChannelOptions
func NewChannel(name, namespace string, o ...ChannelOption) *v1beta1.Channel {
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
func WithInitChannelConditions(c *v1beta1.Channel) {
	c.Status.InitializeConditions()
}

func WithNoAnnotations(c *v1beta1.Channel) {
	c.ObjectMeta.Annotations = nil
}

func WithChannelGeneration(gen int64) ChannelOption {
	return func(s *v1beta1.Channel) {
		s.Generation = gen
	}
}
func WithChannelObservedGeneration(gen int64) ChannelOption {
	return func(s *v1beta1.Channel) {
		s.Status.ObservedGeneration = gen
	}
}

func WithChannelDeleted(c *v1beta1.Channel) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithChannelTemplate(typeMeta metav1.TypeMeta) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.Spec.ChannelTemplate = &messagingv1beta1.ChannelTemplateSpec{
			TypeMeta: typeMeta,
		}
	}
}

func WithBackingChannelFailed(reason, msg string) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.Status.MarkBackingChannelFailed(reason, msg)
	}
}

func WithBackingChannelUnknown(reason, msg string) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.Status.MarkBackingChannelUnknown(reason, msg)
	}
}

func WithBackingChannelReady(c *v1beta1.Channel) {
	c.Status.MarkBackingChannelReady()
}

func WithBackingChannelObjRef(objRef *duckv1.KReference) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.Status.Channel = objRef
	}
}

func WithChannelNoAddress() ChannelOption {
	return func(c *v1beta1.Channel) {
		c.Status.SetAddress(nil)
	}
}

func WithChannelAddress(hostname string) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.Status.SetAddress(&duckv1.Addressable{URL: apis.HTTP(hostname)})
	}
}

func WithChannelReadySubscriber(uid string) ChannelOption {
	return WithChannelReadySubscriberAndGeneration(uid, 0)
}

func WithChannelReadySubscriberAndGeneration(uid string, observedGeneration int64) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.Status.Subscribers = append(c.Status.Subscribers, eventingduckv1beta1.SubscriberStatus{
			UID:                types.UID(uid),
			ObservedGeneration: observedGeneration,
			Ready:              v1.ConditionTrue,
		})
	}
}

func WithChannelSubscriberStatuses(subscriberStatuses []eventingduckv1beta1.SubscriberStatus) ChannelOption {
	return func(c *v1beta1.Channel) {
		c.Status.Subscribers = subscriberStatuses
	}
}
