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

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
)

// ChannelReference references a Channel within the cluster by name and
// namespace.
type ChannelReference struct {
	Namespace string
	Name      string
}

// NewChannelReference creates a ChannelReference from a Channel
func NewChannelReference(channel *eventingv1alpha1.Channel) ChannelReference {
	return NewChannelReferenceFromNames(channel.Name, channel.Namespace)
}

// NewChannelReferenceFromSubscription creates a ChannelReference from a
// Subscription for a Channel.
func NewChannelReferenceFromSubscription(subscription *eventingv1alpha1.Subscription) ChannelReference {
	return NewChannelReferenceFromNames(subscription.Spec.Channel.Name, subscription.Namespace)
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
