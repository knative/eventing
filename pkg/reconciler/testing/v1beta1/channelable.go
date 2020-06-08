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
	"knative.dev/eventing/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/pkg/apis/messaging"
	"knative.dev/pkg/apis"
	pkgduckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	pkgduckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// Channelable allows us to have a fake channel for testing that implements a v1alpha1.Channelable type.

// ChannelableOption enables further configuration of a v1alpha1.Channelable.
type ChannelableOption func(*v1alpha1.Channelable)

// NewChannelable creates an Channelable with ChannelableOptions.
func NewChannelable(name, namespace string, imcopt ...ChannelableOption) *v1alpha1.Channelable {
	c := &v1alpha1.Channelable{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: map[string]string{messaging.SubscribableDuckVersionAnnotation: "v1alpha1"},
		},
		Spec: v1alpha1.ChannelableSpec{},
	}
	for _, opt := range imcopt {
		opt(c)
	}
	return c
}

func WithChannelableGeneration(gen int64) ChannelableOption {
	return func(s *v1alpha1.Channelable) {
		s.Generation = gen
	}
}

func WithChannelableStatusObservedGeneration(gen int64) ChannelableOption {
	return func(s *v1alpha1.Channelable) {
		s.Status.ObservedGeneration = gen
	}
}

func WithChannelableDeleted(imc *v1alpha1.Channelable) {
	deleteTime := metav1.NewTime(time.Unix(1e9, 0))
	imc.ObjectMeta.SetDeletionTimestamp(&deleteTime)
}

func WithChannelableSubscribers(subscribers []v1alpha1.SubscriberSpec) ChannelableOption {
	return func(c *v1alpha1.Channelable) {
		c.Spec.Subscribable = &v1alpha1.Subscribable{Subscribers: subscribers}
	}
}

func WithChannelableReadySubscriber(uid string) ChannelableOption {
	return WithChannelableReadySubscriberAndGeneration(uid, 0)
}

func WithChannelableReadySubscriberAndGeneration(uid string, observedGeneration int64) ChannelableOption {
	return func(c *v1alpha1.Channelable) {
		if c.Status.GetSubscribableTypeStatus() == nil { // Both the SubscribableStatus fields are nil
			c.Status.SetSubscribableTypeStatus(v1alpha1.SubscribableStatus{})
		}
		c.Status.SubscribableTypeStatus.AddSubscriberToSubscribableStatus(duckv1beta1.SubscriberStatus{
			UID:                types.UID(uid),
			ObservedGeneration: observedGeneration,
			Ready:              corev1.ConditionTrue,
		})
	}
}

func WithChannelableStatusSubscribers(subscriberStatuses []duckv1beta1.SubscriberStatus) ChannelableOption {
	return func(c *v1alpha1.Channelable) {
		c.Status.SetSubscribableTypeStatus(v1alpha1.SubscribableStatus{
			Subscribers: subscriberStatuses})
	}
}

func WithChannelableReady() ChannelableOption {
	return func(c *v1alpha1.Channelable) {
		c.Status.Conditions = []apis.Condition{{Type: apis.ConditionReady, Status: corev1.ConditionTrue}}
	}
}

func WithChannelableAddress(a string) ChannelableOption {
	return func(c *v1alpha1.Channelable) {
		c.Status.Address = &pkgduckv1alpha1.Addressable{
			Addressable: pkgduckv1beta1.Addressable{
				URL: apis.HTTP(a),
			},
		}
	}
}
