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

package lib

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/reconciler"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	flowsv1 "knative.dev/eventing/pkg/apis/flows/v1"
	flowsv1beta1 "knative.dev/eventing/pkg/apis/flows/v1beta1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"
	sourcesv1beta1 "knative.dev/eventing/pkg/apis/sources/v1beta1"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
)

// TODO(chizhg): break this file into multiple files when it grows too large.

var coreAPIGroup = corev1.SchemeGroupVersion.Group
var coreAPIVersion = corev1.SchemeGroupVersion.Version
var rbacAPIGroup = rbacv1.SchemeGroupVersion.Group
var rbacAPIVersion = rbacv1.SchemeGroupVersion.Version

// This is a workaround for https://github.com/knative/pkg/issues/1509
// Because tests currently fail immediately on any creation failure, this
// is problematic. On the reconcilers it's not an issue because they recover,
// but tests need this retry.
//
// https://github.com/knative/eventing/issues/3681
func isWebhookError(err error) bool {
	return strings.Contains(err.Error(), "eventing-webhook.knative-eventing")
}

func (c *Client) RetryWebhookErrors(updater func(int) error) error {
	attempts := 0
	return retry.OnError(retry.DefaultRetry, isWebhookError, func() error {
		err := updater(attempts)
		attempts++
		return err
	})
}

// CreateChannelOrFail will create a typed Channel Resource in Eventing or fail the test if there is an error.
func (c *Client) CreateChannelOrFail(name string, channelTypeMeta *metav1.TypeMeta) {
	c.T.Logf("Creating channel %+v-%s", channelTypeMeta, name)
	namespace := c.Namespace
	metaResource := resources.NewMetaResource(name, namespace, channelTypeMeta)
	var gvr schema.GroupVersionResource
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		var e error
		gvr, e = duck.CreateGenericChannelObject(c.Dynamic, metaResource)
		if e != nil {
			c.T.Logf("Failed to create %q %q: %v", channelTypeMeta.Kind, name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create %q %q: %v", channelTypeMeta.Kind, name, err)
	}
	c.Tracker.Add(gvr.Group, gvr.Version, gvr.Resource, namespace, name)
}

// CreateChannelsOrFail will create a list of typed Channel Resources in Eventing or fail the test if there is an error.
func (c *Client) CreateChannelsOrFail(names []string, channelTypeMeta *metav1.TypeMeta) {
	c.T.Logf("Creating channels %+v-%v", channelTypeMeta, names)
	for _, name := range names {
		c.CreateChannelOrFail(name, channelTypeMeta)
	}
}

// CreateChannelWithDefaultOrFail will create a default Channel Resource in Eventing or fail the test if there is an error.
func (c *Client) CreateChannelWithDefaultOrFail(channel *messagingv1beta1.Channel) {
	c.T.Logf("Creating default v1beta1 channel %+v", channel)
	channels := c.Eventing.MessagingV1beta1().Channels(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := channels.Create(channel)
		if e != nil {
			c.T.Logf("Failed to create channel %q: %v", channel.Name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create channel %q: %v", channel.Name, err)
	}
	c.Tracker.AddObj(channel)
}

// CreateChannelV1WithDefaultOrFail will create a default Channel Resource in Eventing or fail the test if there is an error.
func (c *Client) CreateChannelV1WithDefaultOrFail(channel *messagingv1.Channel) {
	c.T.Logf("Creating default v1 channel %+v", channel)
	channels := c.Eventing.MessagingV1().Channels(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := channels.Create(channel)
		if e != nil {
			c.T.Logf("Failed to create channel %q: %v", channel.Name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create channel %q: %v", channel.Name, err)
	}
	c.Tracker.AddObj(channel)
}

// CreateSubscriptionOrFail will create a v1beta1 Subscription or fail the test if there is an error.
func (c *Client) CreateSubscriptionOrFail(
	name, channelName string,
	channelTypeMeta *metav1.TypeMeta,
	options ...resources.SubscriptionOptionV1Beta1,
) *messagingv1beta1.Subscription {
	namespace := c.Namespace
	subscription := resources.SubscriptionV1Beta1(name, channelName, channelTypeMeta, options...)
	subscriptions := c.Eventing.MessagingV1beta1().Subscriptions(namespace)
	var retSubscription *messagingv1beta1.Subscription
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		c.T.Logf("Creating v1beta1 subscription %s for channel %+v-%s", name, channelTypeMeta, channelName)
		// update subscription with the new reference
		var e error
		retSubscription, e = subscriptions.Create(subscription)
		if e != nil {
			c.T.Logf("Failed to create subscription %q: %v", name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create subscription %q: %v", name, err)
	}
	// Note that if the Create above failed with 'already created', then retSubscription won't be valid, so we have to grab it again.
	if err != nil && errors.IsAlreadyExists(err) {
		err = c.RetryWebhookErrors(func(attempts int) (err error) {
			c.T.Logf("Getting v1beta1 subscription %s for channel %+v-%s", name, channelTypeMeta, channelName)
			// update subscription with the new reference
			var e error
			retSubscription, e = subscriptions.Get(name, metav1.GetOptions{})
			if e != nil {
				c.T.Logf("Failed to get subscription %q: %v", name, e)
			}
			return e
		})
		if err != nil {
			c.T.Fatalf("Failed to get a created subscription %q: %v", name, err)
		}
	}
	c.Tracker.AddObj(retSubscription)
	return retSubscription
}

// CreateSubscriptionV1OrFail will create a v1 Subscription or fail the test if there is an error.
func (c *Client) CreateSubscriptionV1OrFail(
	name, channelName string,
	channelTypeMeta *metav1.TypeMeta,
	options ...resources.SubscriptionOptionV1,
) *messagingv1.Subscription {
	namespace := c.Namespace
	subscription := resources.SubscriptionV1(name, channelName, channelTypeMeta, options...)
	var retSubscription *messagingv1.Subscription
	subscriptions := c.Eventing.MessagingV1().Subscriptions(namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		c.T.Logf("Creating v1 subscription %s for channel %+v-%s", name, channelTypeMeta, channelName)
		// update subscription with the new reference
		var e error
		retSubscription, e = subscriptions.Create(subscription)
		if e != nil {
			c.T.Logf("Failed to create subscription %q: %v", name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create subscription %q: %v", name, err)
	}
	if err != nil && errors.IsAlreadyExists(err) {
		err = c.RetryWebhookErrors(func(attempts int) (err error) {
			c.T.Logf("Getting v1 subscription %s for channel %+v-%s", name, channelTypeMeta, channelName)
			// update subscription with the new reference
			var e error
			retSubscription, e = subscriptions.Get(name, metav1.GetOptions{})
			if e != nil {
				c.T.Logf("Failed to create subscription %q: %v", name, e)
			}
			return e
		})
		if err != nil {
			c.T.Fatalf("Failed to get a created subscription %q: %v", name, err)
		}
	}
	c.Tracker.AddObj(retSubscription)
	return retSubscription
}

// CreateSubscriptionsOrFail will create a list of v1beta1 Subscriptions with the same configuration except the name.
func (c *Client) CreateSubscriptionsOrFail(
	names []string,
	channelName string,
	channelTypeMeta *metav1.TypeMeta,
	options ...resources.SubscriptionOptionV1Beta1,
) {
	c.T.Logf("Creating subscriptions %v for channel %+v-%s", names, channelTypeMeta, channelName)
	for _, name := range names {
		c.CreateSubscriptionOrFail(name, channelName, channelTypeMeta, options...)
	}
}

// CreateSubscriptionsV1OrFail will create a list of v1 Subscriptions with the same configuration except the name.
func (c *Client) CreateSubscriptionsV1OrFail(
	names []string,
	channelName string,
	channelTypeMeta *metav1.TypeMeta,
	options ...resources.SubscriptionOptionV1,
) {
	c.T.Logf("Creating subscriptions %v for channel %+v-%s", names, channelTypeMeta, channelName)
	for _, name := range names {
		c.CreateSubscriptionV1OrFail(name, channelName, channelTypeMeta, options...)
	}
}

// CreateConfigMapOrFail will create a configmap or fail the test if there is an error.
func (c *Client) CreateConfigMapOrFail(name, namespace string, data map[string]string) *corev1.ConfigMap {
	c.T.Logf("Creating configmap %s", name)
	configMap, err := c.Kube.Kube.CoreV1().ConfigMaps(namespace).Create(resources.ConfigMap(name, namespace, data))
	if err != nil {
		c.T.Fatalf("Failed to create configmap %s: %v", name, err)
	}
	c.Tracker.Add(coreAPIGroup, coreAPIVersion, "configmaps", namespace, name)
	return configMap
}

func (c *Client) CreateBrokerConfigMapOrFail(name string, channel *metav1.TypeMeta) *duckv1.KReference {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: c.Namespace,
		},
		Data: map[string]string{
			"channelTemplateSpec": fmt.Sprintf(`
      apiVersion: %q
      kind: %q
`, channel.APIVersion, channel.Kind),
		},
	}
	_, err := c.Kube.Kube.CoreV1().ConfigMaps(c.Namespace).Create(cm)
	if err != nil {
		c.T.Fatalf("Failed to create broker config %q: %v", name, err)
	}
	return &duckv1.KReference{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Namespace:  c.Namespace,
		Name:       name,
	}
}

// CreateBrokerV1Beta1OrFail will create a v1beta1 Broker or fail the test if there is an error.
func (c *Client) CreateBrokerV1Beta1OrFail(name string, options ...resources.BrokerV1Beta1Option) *v1beta1.Broker {
	namespace := c.Namespace
	broker := resources.BrokerV1Beta1(name, options...)
	var retBroker *v1beta1.Broker
	brokers := c.Eventing.EventingV1beta1().Brokers(namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		c.T.Logf("Creating v1beta1 broker %s", name)
		// update broker with the new reference
		var e error
		retBroker, e = brokers.Create(broker)
		if e != nil {
			c.T.Logf("Failed to create v1beta1 broker %q: %v", name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create v1beta1 broker %q: %v", name, err)
	}
	if err != nil && errors.IsAlreadyExists(err) {
		err = c.RetryWebhookErrors(func(attempts int) (err error) {
			c.T.Logf("Getting v1beta1 broker %s", name)
			// update broker with the new reference
			var e error
			retBroker, e = brokers.Get(name, metav1.GetOptions{})
			if e != nil {
				c.T.Fatalf("Failed to get created v1beta1 broker %q: %v", name, e)
			}
			return e
		})
	}
	c.Tracker.AddObj(retBroker)
	return retBroker
}

// CreateTriggerOrFailV1Beta1 will create a v1beta1 Trigger or fail the test if there is an error.
func (c *Client) CreateTriggerOrFailV1Beta1(name string, options ...resources.TriggerOptionV1Beta1) *v1beta1.Trigger {
	namespace := c.Namespace
	trigger := resources.TriggerV1Beta1(name, options...)
	var retTrigger *v1beta1.Trigger
	triggers := c.Eventing.EventingV1beta1().Triggers(namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		c.T.Logf("Creating v1beta1 trigger %s", name)
		// update trigger with the new reference
		var e error
		retTrigger, e = triggers.Create(trigger)
		if e != nil {
			c.T.Logf("Failed to create v1beta1 trigger %q: %v", name, e)
		}
		return e
	})

	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create v1beta1 trigger %q: %v", name, err)
	}

	if err != nil && errors.IsAlreadyExists(err) {
		err = c.RetryWebhookErrors(func(attempts int) (err error) {
			c.T.Logf("Getting v1beta1 trigger %s", name)
			// update trigger with the new reference
			var e error
			retTrigger, e = triggers.Get(name, metav1.GetOptions{})
			if e != nil {
				c.T.Logf("Failed to get created v1beta1 trigger %q: %v", name, e)
			}
			return e
		})
		if err != nil {
			c.T.Fatalf("Failed to get created v1beta1 trigger %q: %v", name, err)
		}
	}

	c.Tracker.AddObj(retTrigger)
	return retTrigger
}

// CreateBrokerV1OrFail will create a v1 Broker or fail the test if there is an error.
func (c *Client) CreateBrokerV1OrFail(name string, options ...resources.BrokerV1Option) *eventingv1.Broker {
	namespace := c.Namespace
	broker := resources.BrokerV1(name, options...)
	var retBroker *eventingv1.Broker
	brokers := c.Eventing.EventingV1().Brokers(namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		c.T.Logf("Creating v1 broker %s", name)
		// update broker with the new reference
		var e error
		retBroker, e = brokers.Create(broker)
		if e != nil {
			c.T.Logf("Failed to create v1 broker %q: %v", name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create v1 broker %q: %v", name, err)
	}

	if err != nil && errors.IsAlreadyExists(err) {
		err := c.RetryWebhookErrors(func(attempts int) (err error) {
			c.T.Logf("Getting v1 broker %s", name)
			// update broker with the new reference
			var e error
			retBroker, e = brokers.Get(name, metav1.GetOptions{})
			if e != nil {
				c.T.Logf("Failed to get created v1 broker %q: %v", name, e)
			}
			return e
		})
		if err != nil {
			c.T.Fatalf("Failed to get created v1 broker %q: %v", name, err)
		}
	}
	c.Tracker.AddObj(retBroker)
	return retBroker
}

// CreateTriggerOrFailV1 will create a v1 Trigger or fail the test if there is an error.
func (c *Client) CreateTriggerV1OrFail(name string, options ...resources.TriggerOptionV1) *eventingv1.Trigger {
	namespace := c.Namespace
	trigger := resources.TriggerV1(name, options...)
	var retTrigger *eventingv1.Trigger
	triggers := c.Eventing.EventingV1().Triggers(namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		c.T.Logf("Creating v1 trigger %s", name)
		// update trigger with the new reference
		var e error
		retTrigger, e = triggers.Create(trigger)
		if e != nil {
			c.T.Logf("Failed to create v1 trigger %q: %v", name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create v1 trigger %q: %v", name, err)
	}
	if err != nil && !errors.IsAlreadyExists(err) {
		err = c.RetryWebhookErrors(func(attempts int) (err error) {
			c.T.Logf("Getting v1 trigger %s", name)
			// update trigger with the new reference
			var e error
			retTrigger, e = triggers.Get(name, metav1.GetOptions{})
			if e != nil {
				c.T.Logf("Failed to get created v1 trigger %q: %v", name, e)
			}
			return e
		})
	}
	if err != nil {
		c.T.Fatalf("Failed to get created v1 trigger %q: %v", name, err)
	}
	c.Tracker.AddObj(retTrigger)
	return retTrigger
}

// CreateFlowsSequenceOrFail will create a Sequence (in flows.knative.dev api group) or
// fail the test if there is an error.
func (c *Client) CreateFlowsSequenceOrFail(sequence *flowsv1beta1.Sequence) {
	c.T.Logf("Creating flows sequence %+v", sequence)
	sequences := c.Eventing.FlowsV1beta1().Sequences(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := sequences.Create(sequence)
		if e != nil {
			c.T.Logf("Failed to create flows sequence %q: %v", sequence.Name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create flows sequence %q: %v", sequence.Name, err)
	}
	c.Tracker.AddObj(sequence)
}

// CreateFlowsSequenceOrFail will create a Sequence (in flows.knative.dev api group) or
// fail the test if there is an error.
func (c *Client) CreateFlowsSequenceV1OrFail(sequence *flowsv1.Sequence) {
	c.T.Logf("Creating flows sequence %+v", sequence)
	sequences := c.Eventing.FlowsV1().Sequences(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := sequences.Create(sequence)
		if e != nil {
			c.T.Logf("Failed to create flows sequence %q: %v", sequence.Name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create flows sequence %q: %v", sequence.Name, err)
	}
	c.Tracker.AddObj(sequence)
}

// CreateFlowsParallelOrFail will create a Parallel (in flows.knative.dev api group) or
// fail the test if there is an error.
func (c *Client) CreateFlowsParallelOrFail(parallel *flowsv1beta1.Parallel) {
	c.T.Logf("Creating flows parallel %+v", parallel)
	parallels := c.Eventing.FlowsV1beta1().Parallels(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := parallels.Create(parallel)
		if e != nil {
			c.T.Logf("Failed to create flows parallel %q: %v", parallel.Name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create flows parallel %q: %v", parallel.Name, err)
	}
	c.Tracker.AddObj(parallel)
}

// CreateFlowsParallelOrFail will create a Parallel (in flows.knative.dev api group) or
// fail the test if there is an error.
func (c *Client) CreateFlowsParallelV1OrFail(parallel *flowsv1.Parallel) {
	c.T.Logf("Creating flows parallel %+v", parallel)
	parallels := c.Eventing.FlowsV1().Parallels(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := parallels.Create(parallel)
		if e != nil {
			c.T.Logf("Failed to create flows parallel %q: %v", parallel.Name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create flows parallel %q: %v", parallel.Name, err)
	}
	c.Tracker.AddObj(parallel)
}

// CreateSinkBindingV1Alpha1OrFail will create a SinkBinding or fail the test if there is an error.
func (c *Client) CreateSinkBindingV1Alpha1OrFail(sb *sourcesv1alpha1.SinkBinding) {
	c.T.Logf("Creating sinkbinding %+v", sb)
	sbInterface := c.Eventing.SourcesV1alpha1().SinkBindings(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := sbInterface.Create(sb)
		if e != nil {
			c.T.Logf("Failed to create sinkbinding %q: %v", sb.Name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create sinkbinding %q: %v", sb.Name, err)
	}
	c.Tracker.AddObj(sb)
}

// CreateSinkBindingV1Alpha2OrFail will create a SinkBinding or fail the test if there is an error.
func (c *Client) CreateSinkBindingV1Alpha2OrFail(sb *sourcesv1alpha2.SinkBinding) {
	c.T.Logf("Creating sinkbinding %+v", sb)
	sbInterface := c.Eventing.SourcesV1alpha2().SinkBindings(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := sbInterface.Create(sb)
		if e != nil {
			c.T.Logf("Failed to create sinkbinding %q: %v", sb.Name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create sinkbinding %q: %v", sb.Name, err)
	}
	c.Tracker.AddObj(sb)
}

// CreateSinkBindingV1Beta1OrFail will create a SinkBinding or fail the test if there is an error.
func (c *Client) CreateSinkBindingV1Beta1OrFail(sb *sourcesv1beta1.SinkBinding) {
	c.T.Logf("Creating sinkbinding %+v", sb)
	sbInterface := c.Eventing.SourcesV1beta1().SinkBindings(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := sbInterface.Create(sb)
		if e != nil {
			c.T.Logf("Failed to create sinkbinding %q: %v", sb.Name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create sinkbinding %q: %v", sb.Name, err)
	}
	c.Tracker.AddObj(sb)
}

// CreateApiServerSourceV1Alpha2OrFail will create an v1alpha2 ApiServerSource
func (c *Client) CreateApiServerSourceV1Alpha2OrFail(apiServerSource *sourcesv1alpha2.ApiServerSource) {
	c.T.Logf("Creating apiserversource %+v", apiServerSource)
	apiServerInterface := c.Eventing.SourcesV1alpha2().ApiServerSources(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := apiServerInterface.Create(apiServerSource)
		if e != nil {
			c.T.Logf("Failed to create apiserversource %q: %v", apiServerSource.Name, err)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create apiserversource %q: %v", apiServerSource.Name, err)
	}
	c.Tracker.AddObj(apiServerSource)
}

// CreateApiServerSourceV1Beta1OrFail will create an v1beta1 ApiServerSource
func (c *Client) CreateApiServerSourceV1Beta1OrFail(apiServerSource *sourcesv1beta1.ApiServerSource) {
	c.T.Logf("Creating apiserversource %+v", apiServerSource)
	apiServerInterface := c.Eventing.SourcesV1beta1().ApiServerSources(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := apiServerInterface.Create(apiServerSource)
		if e != nil {
			c.T.Logf("Failed to create apiserversource %q: %v", apiServerSource.Name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create apiserversource %q: %v", apiServerSource.Name, err)
	}
	c.Tracker.AddObj(apiServerSource)
}

// CreateContainerSourceV1Alpha2OrFail will create a v1alpha2 ContainerSource.
func (c *Client) CreateContainerSourceV1Alpha2OrFail(containerSource *sourcesv1alpha2.ContainerSource) {
	c.T.Logf("Creating containersource %+v", containerSource)
	containerInterface := c.Eventing.SourcesV1alpha2().ContainerSources(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := containerInterface.Create(containerSource)
		if e != nil {
			c.T.Logf("Failed to create containersource %q: %v", containerSource.Name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create containersource %q: %v", containerSource.Name, err)
	}
	c.Tracker.AddObj(containerSource)
}

// CreateContainerSourceV1Beta1OrFail will create a v1beta1 ContainerSource.
func (c *Client) CreateContainerSourceV1Beta1OrFail(containerSource *sourcesv1beta1.ContainerSource) {
	c.T.Logf("Creating containersource %+v", containerSource)
	containerInterface := c.Eventing.SourcesV1beta1().ContainerSources(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := containerInterface.Create(containerSource)
		if e != nil {
			c.T.Logf("Failed to create containersource %q: %v", containerSource.Name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create containersource %q: %v", containerSource.Name, err)
	}
	c.Tracker.AddObj(containerSource)
}

// CreatePingSourceV1Alpha2OrFail will create an PingSource
func (c *Client) CreatePingSourceV1Alpha2OrFail(pingSource *sourcesv1alpha2.PingSource) {
	c.T.Logf("Creating pingsource %+v", pingSource)
	pingInterface := c.Eventing.SourcesV1alpha2().PingSources(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := pingInterface.Create(pingSource)
		if e != nil {
			c.T.Logf("Failed to create pingsource %q: %v", pingSource.Name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create pingsource %q: %v", pingSource.Name, err)
	}
	c.Tracker.AddObj(pingSource)
}

// CreatePingSourceV1Beta1OrFail will create an PingSource
func (c *Client) CreatePingSourceV1Beta1OrFail(pingSource *sourcesv1beta1.PingSource) {
	c.T.Logf("Creating pingsource %+v", pingSource)
	pingInterface := c.Eventing.SourcesV1beta1().PingSources(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := pingInterface.Create(pingSource)
		if e != nil {
			c.T.Logf("Failed to create pingsource %q: %v", pingSource.Name, e)
		}
		return e
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create pingsource %q: %v", pingSource.Name, err)
	}
	c.Tracker.AddObj(pingSource)
}

func (c *Client) CreateServiceOrFail(svc *corev1.Service) *corev1.Service {
	c.T.Logf("Creating service %+v", svc)
	namespace := c.Namespace
	if newSvc, err := c.Kube.Kube.CoreV1().Services(namespace).Create(svc); err != nil {
		c.T.Fatalf("Failed to create service %q: %v", svc.Name, err)
		return nil
	} else {
		c.Tracker.Add(coreAPIGroup, coreAPIVersion, "services", namespace, svc.Name)
		return newSvc
	}
}

// WithService returns an option that creates a Service binded with the given pod.
func WithService(name string) func(*corev1.Pod, *Client) error {
	return func(pod *corev1.Pod, client *Client) error {
		namespace := pod.Namespace
		svc := resources.ServiceDefaultHTTP(name, pod.Labels)

		svcs := client.Kube.Kube.CoreV1().Services(namespace)
		if _, err := svcs.Create(svc); err != nil {
			return err
		}
		client.Tracker.Add(coreAPIGroup, coreAPIVersion, "services", namespace, name)
		return nil
	}
}

// CreatePodOrFail will create a Pod or fail the test if there is an error.
func (c *Client) CreatePodOrFail(pod *corev1.Pod, options ...func(*corev1.Pod, *Client) error) {
	// set namespace for the pod in case it's empty
	namespace := c.Namespace
	pod.Namespace = namespace
	// apply options on the pod before creation
	for _, option := range options {
		if err := option(pod, c); err != nil {
			c.T.Fatalf("Failed to configure pod %q: %v", pod.Name, err)
		}
	}

	c.applyTracingEnv(&pod.Spec)

	err := reconciler.RetryUpdateConflicts(func(attempts int) (err error) {
		c.T.Logf("Creating pod %+v", pod)
		_, e := c.Kube.CreatePod(pod)
		return e
	})
	if err != nil {
		c.T.Fatalf("Failed to create pod %q: %v", pod.Name, err)
	}
	c.Tracker.Add(coreAPIGroup, coreAPIVersion, "pods", namespace, pod.Name)
	c.podsCreated = append(c.podsCreated, pod.Name)
}

// GetServiceHost returns the service hostname for the specified podName
func (c *Client) GetServiceHost(podName string) string {
	return fmt.Sprintf("%s.%s.svc", podName, c.Namespace)
}

// CreateDeploymentOrFail will create a Deployment or fail the test if there is an error.
func (c *Client) CreateDeploymentOrFail(deploy *appsv1.Deployment, options ...func(*appsv1.Deployment, *Client) error) {
	// set namespace for the deploy in case it's empty
	namespace := c.Namespace
	deploy.Namespace = namespace
	// apply options on the deploy before creation
	for _, option := range options {
		if err := option(deploy, c); err != nil {
			c.T.Fatalf("Failed to configure deploy %q: %v", deploy.Name, err)
		}
	}

	c.applyTracingEnv(&deploy.Spec.Template.Spec)

	c.T.Logf("Creating deployment %+v", deploy)
	if _, err := c.Kube.Kube.AppsV1().Deployments(deploy.Namespace).Create(deploy); err != nil {
		c.T.Fatalf("Failed to create deploy %q: %v", deploy.Name, err)
	}
	c.Tracker.Add("apps", "v1", "deployments", namespace, deploy.Name)
}

// CreateCronJobOrFail will create a CronJob or fail the test if there is an error.
func (c *Client) CreateCronJobOrFail(cronjob *batchv1beta1.CronJob, options ...func(*batchv1beta1.CronJob, *Client) error) {
	// set namespace for the cronjob in case it's empty
	namespace := c.Namespace
	cronjob.Namespace = namespace
	// apply options on the cronjob before creation
	for _, option := range options {
		if err := option(cronjob, c); err != nil {
			c.T.Fatalf("Failed to configure cronjob %q: %v", cronjob.Name, err)
		}
	}

	c.applyTracingEnv(&cronjob.Spec.JobTemplate.Spec.Template.Spec)

	c.T.Logf("Creating cronjob %+v", cronjob)
	if _, err := c.Kube.Kube.BatchV1beta1().CronJobs(cronjob.Namespace).Create(cronjob); err != nil {
		c.T.Fatalf("Failed to create cronjob %q: %v", cronjob.Name, err)
	}
	c.Tracker.Add("batch", "v1beta1", "cronjobs", namespace, cronjob.Name)
}

// CreateServiceAccountOrFail will create a ServiceAccount or fail the test if there is an error.
func (c *Client) CreateServiceAccountOrFail(saName string) {
	namespace := c.Namespace
	sa := resources.ServiceAccount(saName, namespace)
	sas := c.Kube.Kube.CoreV1().ServiceAccounts(namespace)
	c.T.Logf("Creating service account %+v", sa)
	if _, err := sas.Create(sa); err != nil {
		c.T.Fatalf("Failed to create service account %q: %v", saName, err)
	}
	c.Tracker.Add(coreAPIGroup, coreAPIVersion, "serviceaccounts", namespace, saName)

	// If the "default" Namespace has a secret called
	// "kn-eventing-test-pull-secret" then use that as the ImagePullSecret
	// on the new ServiceAccount we just created.
	// This is needed for cases where the images are in a private registry.
	_, err := utils.CopySecret(c.Kube.Kube.CoreV1(), "default", testPullSecretName, namespace, saName)
	if err != nil && !errors.IsNotFound(err) {
		c.T.Fatalf("Error copying the secret: %s", err)
	}
}

// CreateClusterRoleOrFail creates the given ClusterRole or fail the test if there is an error.
func (c *Client) CreateClusterRoleOrFail(cr *rbacv1.ClusterRole) {
	c.T.Logf("Creating cluster role %+v", cr)
	crs := c.Kube.Kube.RbacV1().ClusterRoles()
	if _, err := crs.Create(cr); err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create cluster role %q: %v", cr.Name, err)
	}
	c.Tracker.Add(rbacAPIGroup, rbacAPIVersion, "clusterroles", "", cr.Name)
}

// CreateRoleOrFail creates the given Role in the Client namespace or fail the test if there is an error.
func (c *Client) CreateRoleOrFail(r *rbacv1.Role) {
	c.T.Logf("Creating role %+v", r)
	namespace := c.Namespace
	rs := c.Kube.Kube.RbacV1().Roles(namespace)
	if _, err := rs.Create(r); err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create role %q: %v", r.Name, err)
	}
	c.Tracker.Add(rbacAPIGroup, rbacAPIVersion, "roles", namespace, r.Name)
}

const (
	ClusterRoleKind = "ClusterRole"
	RoleKind        = "Role"
)

// CreateRoleBindingOrFail will create a RoleBinding or fail the test if there is an error.
func (c *Client) CreateRoleBindingOrFail(saName, rKind, rName, rbName, rbNamespace string) {
	saNamespace := c.Namespace
	rb := resources.RoleBinding(saName, saNamespace, rKind, rName, rbName, rbNamespace)
	rbs := c.Kube.Kube.RbacV1().RoleBindings(rbNamespace)

	c.T.Logf("Creating role binding %+v", rb)
	if _, err := rbs.Create(rb); err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create role binding %q: %v", rbName, err)
	}
	c.Tracker.Add(rbacAPIGroup, rbacAPIVersion, "rolebindings", rbNamespace, rb.GetName())
}

// CreateClusterRoleBindingOrFail will create a ClusterRoleBinding or fail the test if there is an error.
func (c *Client) CreateClusterRoleBindingOrFail(saName, crName, crbName string) {
	saNamespace := c.Namespace
	crb := resources.ClusterRoleBinding(saName, saNamespace, crName, crbName)
	crbs := c.Kube.Kube.RbacV1().ClusterRoleBindings()

	c.T.Logf("Creating cluster role binding %+v", crb)
	if _, err := crbs.Create(crb); err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create cluster role binding %q: %v", crbName, err)
	}
	c.Tracker.Add(rbacAPIGroup, rbacAPIVersion, "clusterrolebindings", "", crb.GetName())
}

const (
	// the two ServiceAccounts are required for creating new Brokers in the current namespace
	saIngressName = "eventing-broker-ingress"
	saFilterName  = "eventing-broker-filter"

	// the three ClusterRoles are preinstalled in Knative Eventing setup
	crIngressName      = "eventing-broker-ingress"
	crFilterName       = "eventing-broker-filter"
	crConfigReaderName = "eventing-config-reader"
)

// CreateRBACResourcesForBrokers creates required RBAC resources for creating Brokers,
// see https://github.com/knative/docs/blob/master/docs/eventing/broker-trigger.md - Manual Setup.
func (c *Client) CreateRBACResourcesForBrokers() {
	c.CreateServiceAccountOrFail(saIngressName)
	c.CreateServiceAccountOrFail(saFilterName)
	// The two RoleBindings are required for running Brokers correctly.
	c.CreateRoleBindingOrFail(
		saIngressName,
		ClusterRoleKind,
		crIngressName,
		fmt.Sprintf("%s-%s", saIngressName, crIngressName),
		c.Namespace,
	)
	c.CreateRoleBindingOrFail(
		saFilterName,
		ClusterRoleKind,
		crFilterName,
		fmt.Sprintf("%s-%s", saFilterName, crFilterName),
		c.Namespace,
	)
}

func (c *Client) applyTracingEnv(pod *corev1.PodSpec) {
	for i := 0; i < len(pod.Containers); i++ {
		pod.Containers[i].Env = append(pod.Containers[i].Env, c.tracingEnv)
	}
}
