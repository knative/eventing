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

// Code generated by client-gen. DO NOT EDIT.

package v1beta1

import (
	rest "k8s.io/client-go/rest"
	v1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/eventing/pkg/client/clientset/versioned/scheme"
)

type MessagingV1beta1Interface interface {
	RESTClient() rest.Interface
	ChannelsGetter
	SubscriptionsGetter
}

// MessagingV1beta1Client is used to interact with features provided by the messaging.knative.dev group.
type MessagingV1beta1Client struct {
	restClient rest.Interface
}

func (c *MessagingV1beta1Client) Channels(namespace string) ChannelInterface {
	return newChannels(c, namespace)
}

func (c *MessagingV1beta1Client) Subscriptions(namespace string) SubscriptionInterface {
	return newSubscriptions(c, namespace)
}

// NewForConfig creates a new MessagingV1beta1Client for the given config.
func NewForConfig(c *rest.Config) (*MessagingV1beta1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &MessagingV1beta1Client{client}, nil
}

// NewForConfigOrDie creates a new MessagingV1beta1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *MessagingV1beta1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new MessagingV1beta1Client for the given RESTClient.
func New(c rest.Interface) *MessagingV1beta1Client {
	return &MessagingV1beta1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1beta1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *MessagingV1beta1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
