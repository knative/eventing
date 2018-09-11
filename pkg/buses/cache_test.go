/*
Copyright 2018 The Knative Authors

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

package buses_test

import (
	"testing"

	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	"github.com/knative/eventing/pkg/buses"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	cacheDefaultNamespace = "default"
	cacheTestChannel      = "test-channel"
	cacheTestSubscription = "test-subscription"
)

func TestCacheErrsForUnknownChannel(t *testing.T) {
	cache := buses.NewCache()
	ref := buses.NewChannelReferenceFromNames(cacheTestChannel, cacheDefaultNamespace)
	var expected *channelsv1alpha1.Channel
	actual, err := cache.Channel(ref)
	if err == nil {
		t.Errorf("%s expected: %+v got: %+v", "Error", "<error>", err)
	}
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "Unexpected channel", nil, actual)
	}
}

func TestCacheRetrievesKnownChannel(t *testing.T) {
	cache := buses.NewCache()
	ref := buses.NewChannelReferenceFromNames(cacheTestChannel, cacheDefaultNamespace)
	expected := makeChannel(ref)
	cache.AddChannel(expected)
	actual, err := cache.Channel(ref)
	if err != nil {
		t.Errorf("%s expected: %+v got: %+v", "Unexpected error", nil, err)
	}
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "Channel", expected, actual)
	}
}

func TestCacheRemovesKnownChannel(t *testing.T) {
	cache := buses.NewCache()
	ref := buses.NewChannelReferenceFromNames(cacheTestChannel, cacheDefaultNamespace)
	channel := makeChannel(ref)
	cache.AddChannel(channel)
	cache.RemoveChannel(channel)
	var expected *channelsv1alpha1.Channel
	actual, err := cache.Channel(ref)
	if err == nil {
		t.Errorf("%s expected: %+v got: %+v", "Unexpected error", nil, err)
	}
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "Channel", expected, actual)
	}
}

func TestCacheNilChannel(t *testing.T) {
	cache := buses.NewCache()
	var channel *channelsv1alpha1.Channel
	cache.AddChannel(channel)
	cache.RemoveChannel(channel)
}

func TestCacheErrsForUnknownSubscription(t *testing.T) {
	cache := buses.NewCache()
	ref := buses.NewSubscriptionReferenceFromNames(cacheTestSubscription, cacheDefaultNamespace)
	var expected *channelsv1alpha1.Subscription
	actual, err := cache.Subscription(ref)
	if err == nil {
		t.Errorf("%s expected: %+v got: %+v", "Error", "<error>", err)
	}
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "Unexpected subscription", nil, actual)
	}
}

func TestCacheRetrievesKnownSubscription(t *testing.T) {
	cache := buses.NewCache()
	ref := buses.NewSubscriptionReferenceFromNames(cacheTestSubscription, cacheDefaultNamespace)
	expected := makeSubscription(ref)
	cache.AddSubscription(expected)
	actual, err := cache.Subscription(ref)
	if err != nil {
		t.Errorf("%s expected: %+v got: %+v", "Unexpected error", nil, err)
	}
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "Subscription", expected, actual)
	}
}

func TestCacheRemovesKnownSubscription(t *testing.T) {
	cache := buses.NewCache()
	ref := buses.NewSubscriptionReferenceFromNames(cacheTestSubscription, cacheDefaultNamespace)
	subscription := makeSubscription(ref)
	cache.AddSubscription(subscription)
	cache.RemoveSubscription(subscription)
	var expected *channelsv1alpha1.Subscription
	actual, err := cache.Subscription(ref)
	if err == nil {
		t.Errorf("%s expected: %+v got: %+v", "Unexpected error", nil, err)
	}
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "Subscription", expected, actual)
	}
}

func TestCacheNilSubscription(t *testing.T) {
	cache := buses.NewCache()
	var subscription *channelsv1alpha1.Subscription
	cache.AddSubscription(subscription)
	cache.RemoveSubscription(subscription)
}

func makeChannel(ref buses.ChannelReference) *channelsv1alpha1.Channel {
	return &channelsv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ref.Name,
			Namespace: ref.Namespace,
		},
	}
}

func makeSubscription(ref buses.SubscriptionReference) *channelsv1alpha1.Subscription {
	return &channelsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ref.Name,
			Namespace: ref.Namespace,
		},
	}
}
