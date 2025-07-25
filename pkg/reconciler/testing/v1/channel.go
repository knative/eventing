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
	"fmt"
	"time"

	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/apis/feature"

	"k8s.io/apimachinery/pkg/types"

	v1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	duckv1 "knative.dev/pkg/apis/duck/v1"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
)

// ChannelOption enables further configuration of a Channel.
type ChannelOption func(*messagingv1.Channel)

// NewChannel creates a Channel with ChannelOptions
func NewChannel(name, namespace string, o ...ChannelOption) *messagingv1.Channel {
	c := &messagingv1.Channel{
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
func WithInitChannelConditions(c *messagingv1.Channel) {
	c.Status.InitializeConditions()
}

func WithNoAnnotations(c *messagingv1.Channel) {
	c.ObjectMeta.Annotations = nil
}

func WithChannelGeneration(gen int64) ChannelOption {
	return func(s *messagingv1.Channel) {
		s.Generation = gen
	}
}

func WithChannelObservedGeneration(gen int64) ChannelOption {
	return func(s *messagingv1.Channel) {
		s.Status.ObservedGeneration = gen
	}
}

func WithChannelDeleted(c *messagingv1.Channel) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithChannelTemplate(typeMeta metav1.TypeMeta) ChannelOption {
	return func(c *messagingv1.Channel) {
		c.Spec.ChannelTemplate = &messagingv1.ChannelTemplateSpec{
			TypeMeta: typeMeta,
		}
	}
}

func WithBackingChannelFailed(reason, msg string) ChannelOption {
	return func(c *messagingv1.Channel) {
		c.Status.MarkBackingChannelFailed(reason, msg)
	}
}

func WithBackingChannelUnknown(reason, msg string) ChannelOption {
	return func(c *messagingv1.Channel) {
		c.Status.MarkBackingChannelUnknown(reason, msg)
	}
}

func WithBackingChannelReady(c *messagingv1.Channel) {
	c.Status.MarkBackingChannelReady()
}

func WithChannelDelivery(d *eventingduckv1.DeliverySpec) ChannelOption {
	return func(c *messagingv1.Channel) {
		c.Spec.Delivery = d
	}
}

func WithBackingChannelObjRef(objRef *duckv1.KReference) ChannelOption {
	return func(c *messagingv1.Channel) {
		c.Status.Channel = objRef
	}
}

func WithChannelNoAddress() ChannelOption {
	return func(c *messagingv1.Channel) {
		c.Status.SetAddress(nil)
	}
}

func WithChannelAddress(addr *duckv1.Addressable) ChannelOption {
	return func(c *messagingv1.Channel) {
		c.Status.SetAddress(addr)
	}
}

func WithChannelReadySubscriber(uid string) ChannelOption {
	return WithChannelReadySubscriberAndGeneration(uid, 0)
}

func WithChannelReadySubscriberAndGeneration(uid string, observedGeneration int64) ChannelOption {
	return func(c *messagingv1.Channel) {
		c.Status.Subscribers = append(c.Status.Subscribers, eventingduckv1.SubscriberStatus{
			UID:                types.UID(uid),
			ObservedGeneration: observedGeneration,
			Ready:              v1.ConditionTrue,
		})
	}
}

func WithChannelSubscriberStatuses(subscriberStatuses []eventingduckv1.SubscriberStatus) ChannelOption {
	return func(c *messagingv1.Channel) {
		c.Status.Subscribers = subscriberStatuses
	}
}

func WithChannelStatusDLS(ds eventingduckv1.DeliveryStatus) ChannelOption {
	return func(c *messagingv1.Channel) {
		c.Status.MarkDeadLetterSinkResolvedSucceeded(ds)
	}
}

func WithChannelDLSUnknown() ChannelOption {
	return func(c *messagingv1.Channel) {
		c.Status.MarkDeadLetterSinkNotConfigured()
	}
}

func WithChannelEventPoliciesReady() ChannelOption {
	return func(c *messagingv1.Channel) {
		c.Status.MarkEventPoliciesTrue()
	}
}

func WithChannelEventPoliciesNotReady(reason, message string) ChannelOption {
	return func(c *messagingv1.Channel) {
		c.Status.MarkEventPoliciesFailed(reason, message)
	}
}

func WithChannelEventPoliciesListed(policyNames ...string) ChannelOption {
	return func(c *messagingv1.Channel) {
		for _, name := range policyNames {
			c.Status.Policies = append(c.Status.Policies, eventingduckv1.AppliedEventPolicyRef{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Name:       name,
			})
		}
	}
}

func WithChannelEventPoliciesReadyBecauseOIDCDisabled() ChannelOption {
	return func(c *messagingv1.Channel) {
		c.Status.MarkEventPoliciesTrueWithReason("OIDCDisabled", "Feature %q must be enabled to support Authorization", feature.OIDCAuthentication)
	}
}

func WithChannelEventPoliciesReadyBecauseNoPolicyAndOIDCEnabled() ChannelOption {
	return func(c *messagingv1.Channel) {
		c.Status.MarkEventPoliciesTrueWithReason("DefaultAuthorizationMode", "Default authz mode is %q", feature.AuthorizationAllowSameNamespace)
	}
}

func WithChannelDLSResolvedFailed() ChannelOption {
	return func(c *messagingv1.Channel) {
		c.Status.MarkDeadLetterSinkResolvedFailed(
			"Unable to get the DeadLetterSink's URI",
			fmt.Sprintf(`services "%s" not found`,
				c.Spec.Delivery.DeadLetterSink.Ref.Name,
			),
		)
	}
}
