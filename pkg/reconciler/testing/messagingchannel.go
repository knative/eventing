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

	v1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eventingduckv1alpha1 "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/pkg/apis"
	duck "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis/duck/v1beta1"
)

// TODO once we remove Channel from eventing, we should rename this to be just Channel.

// MessagingChannelOption enables further configuration of a Channel.
type MessagingChannelOption func(*v1alpha1.Channel)

// NewMessagingChannel creates a Channel with MessagingChannelOptions
func NewMessagingChannel(name, namespace string, o ...MessagingChannelOption) *v1alpha1.Channel {
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

// WithInitMessagingChannelConditions initializes the Channel's conditions.
func WithInitMessagingChannelConditions(c *v1alpha1.Channel) {
	c.Status.InitializeConditions()
}

func WithMessagingChannelDeleted(c *v1alpha1.Channel) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithMessagingChannelTemplate(typeMeta metav1.TypeMeta) MessagingChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Spec.ChannelTemplate = &eventingduckv1alpha1.ChannelTemplateSpec{
			TypeMeta: typeMeta,
		}
	}
}

func WithBackingChannelFailed(reason, msg string) MessagingChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Status.MarkBackingChannelFailed(reason, msg)
	}
}

func WithBackingChannelReady(c *v1alpha1.Channel) {
	c.Status.MarkBackingChannelReady()
}

func WithBackingChannelObjRef(objRef *v1.ObjectReference) MessagingChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Status.Channel = objRef
	}
}

func WithMessagingChannelAddress(hostname string) MessagingChannelOption {
	return func(c *v1alpha1.Channel) {
		address := &duck.Addressable{
			Addressable: v1beta1.Addressable{
				URL: &apis.URL{
					Scheme: "http",
					Host:   hostname,
				},
			},
		}
		c.Status.SetAddress(address)
	}
}

func WithMessagingChannelSubscribers(subscribers []eventingduckv1alpha1.SubscriberSpec) MessagingChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Spec.Subscribable = &eventingduckv1alpha1.Subscribable{
			Subscribers: subscribers,
		}
	}
}

func WithMesssagingChannelSubscriberStatuses(subscriberStatuses []eventingduckv1alpha1.SubscriberStatus) MessagingChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Status.SubscribableStatus = &eventingduckv1alpha1.SubscribableStatus{
			Subscribers: subscriberStatuses,
		}
	}
}
