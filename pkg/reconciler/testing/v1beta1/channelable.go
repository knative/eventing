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
	"time"

	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/messaging"
	"knative.dev/pkg/apis"
)

// Channelable allows us to have a fake channel for testing that implements a v1alpha1.Channelable type.

// ChannelableOption enables further configuration of a v1alpha1.Channelable.
type ChannelableOption func(*duckv1.Channelable)

// NewChannelable creates an Channelable with ChannelableOptions.
func NewChannelable(name, namespace string, imcopt ...ChannelableOption) *duckv1.Channelable {
	c := &duckv1.Channelable{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: map[string]string{messaging.SubscribableDuckVersionAnnotation: "v1alpha1"},
		},
		Spec: duckv1.ChannelableSpec{},
	}
	for _, opt := range imcopt {
		opt(c)
	}
	return c
}

func WithChannelableGeneration(gen int64) ChannelableOption {
	return func(s *duckv1.Channelable) {
		s.Generation = gen
	}
}

func WithChannelableStatusObservedGeneration(gen int64) ChannelableOption {
	return func(s *duckv1.Channelable) {
		s.Status.ObservedGeneration = gen
	}
}

func WithChannelableDeleted(imc *duckv1.Channelable) {
	deleteTime := metav1.NewTime(time.Unix(1e9, 0))
	imc.ObjectMeta.SetDeletionTimestamp(&deleteTime)
}

func WithChannelableSubscribers(subscribers []duckv1.SubscriberSpec) ChannelableOption {
	return func(c *duckv1.Channelable) {
		c.Spec.Subscribers = append(c.Spec.Subscribers, subscribers...)
	}
}

func WithChannelableReadySubscriber(uid string) ChannelableOption {
	return WithChannelableReadySubscriberAndGeneration(uid, 0)
}

func WithChannelableReadySubscriberAndGeneration(uid string, observedGeneration int64) ChannelableOption {
	return func(c *duckv1.Channelable) {
		c.Status.SubscribableStatus.Subscribers = append(c.Status.SubscribableStatus.Subscribers, duckv1.SubscriberStatus{
			UID:                types.UID(uid),
			ObservedGeneration: observedGeneration,
			Ready:              corev1.ConditionTrue,
		})
	}
}

func WithChannelableStatusSubscribers(subscriberStatuses []duckv1.SubscriberStatus) ChannelableOption {
	return func(c *duckv1.Channelable) {
		c.Status.Subscribers = append(c.Status.Subscribers, subscriberStatuses...)
	}
}

func WithChannelableReady() ChannelableOption {
	return func(c *duckv1.Channelable) {
		c.Status.Conditions = []apis.Condition{{Type: apis.ConditionReady, Status: corev1.ConditionTrue}}
	}
}

func WithChannelableAddress(a string) ChannelableOption {
	return func(c *duckv1.Channelable) {
		c.Status.AddressStatus.Address.URL = apis.HTTP(a)
	}
}
