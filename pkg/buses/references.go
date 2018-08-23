/*
 * Copyright 2018 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package buses

import (
	"fmt"

	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
)

// BusReference references a Bus or ClusterBus within the cluster by name and
// namespace. For ClusterBus the namespace will be an empty string.
type BusReference struct {
	Namespace string
	Name      string
}

// NewBusReference creates a BusReference for a bus.
func NewBusReference(bus channelsv1alpha1.GenericBus) BusReference {
	meta := bus.GetObjectMeta()
	return BusReference{
		Namespace: meta.GetNamespace(),
		Name:      meta.GetName(),
	}
}

// NewBusReferenceFromNames creates a BusReference for a name and namespace.
func NewBusReferenceFromNames(name, namespace string) BusReference {
	return BusReference{
		Namespace: namespace,
		Name:      name,
	}
}

func (r *BusReference) String() string {
	if r.Namespace != "" {
		return fmt.Sprintf("%s/%s", r.Namespace, r.Name)
	}
	return r.Name
}

// ChannelReference references a Channel within the cluster by name and
// namespace.
type ChannelReference struct {
	Namespace string
	Name      string
}

// NewChannelReference creates a ChannelReference from a Channel
func NewChannelReference(channel *channelsv1alpha1.Channel) ChannelReference {
	return NewChannelReferenceFromNames(channel.Name, channel.Namespace)
}

// NewChannelReferenceFromSubscription creates a ChannelReference from a
// Subscription for a Channel.
func NewChannelReferenceFromSubscription(subscription *channelsv1alpha1.Subscription) ChannelReference {
	return NewChannelReferenceFromNames(subscription.Spec.Channel, subscription.Namespace)
}

// NewChannelReferenceFromNames creates a ChannelReference for a name and
// namespace.
func NewChannelReferenceFromNames(name, namespace string) ChannelReference {
	return ChannelReference{
		Namespace: namespace,
		Name:      name,
	}
}

func (r *ChannelReference) String() string {
	return fmt.Sprintf("%s/%s", r.Namespace, r.Name)
}

// SubscriptionReference references a Subscription within the cluster by name
// and namespace.
type SubscriptionReference struct {
	Namespace string
	Name      string
}

// NewSubscriptionReference creates a SubscriptionReference from a Subscription
func NewSubscriptionReference(subscription *channelsv1alpha1.Subscription) SubscriptionReference {
	return NewSubscriptionReferenceFromNames(subscription.Name, subscription.Namespace)
}

// NewSubscriptionReferenceFromNames creates a SubscriptionReference for a name and
// namespace.
func NewSubscriptionReferenceFromNames(name, namespace string) SubscriptionReference {
	return SubscriptionReference{
		Namespace: namespace,
		Name:      name,
	}
}

func (r *SubscriptionReference) String() string {
	return fmt.Sprintf("%s/%s", r.Namespace, r.Name)
}
