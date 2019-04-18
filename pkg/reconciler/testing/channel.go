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

	duckv1alpha1 "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChannelOption enables further configuration of a Channel.
type ChannelOption func(*v1alpha1.Channel)

// NewChannel creates a Channel with ChannelOptions
func NewChannel(name, namespace string, o ...ChannelOption) *v1alpha1.Channel {
	c := &v1alpha1.Channel{
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

// NewChannelWithoutNamespace creates a Channel with ChannelOptions but without a specific namespace
func NewChannelWithoutNamespace(name string, o ...ChannelOption) *v1alpha1.Channel {
	s := &v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, opt := range o {
		opt(s)
	}
	s.SetDefaults(context.Background())
	return s
}

// WithInitChannelConditions initializes the Channel's conditions.
func WithInitChannelConditions(s *v1alpha1.Channel) {
	s.Status.InitializeConditions()
}

func WithChannelAddress(hostname string) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Status.Address.Hostname = hostname
	}
}

func WithChannelSubscribers(subscribers []duckv1alpha1.ChannelSubscriberSpec) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Spec.Subscribable = &duckv1alpha1.Subscribable{
			Subscribers: subscribers,
		}
	}
}
