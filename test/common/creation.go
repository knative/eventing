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

package common

import (
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/test/base"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

var coreAPIGroup = corev1.SchemeGroupVersion.Group
var coreAPIVersion = corev1.SchemeGroupVersion.Version
var rbacAPIGroup = rbacv1.SchemeGroupVersion.Group
var rbacAPIVersion = rbacv1.SchemeGroupVersion.Version

// CreateChannel will create a Channel Resource in Eventing.
func (client *Client) CreateChannel(name, provisonerName string) error {
	namespace := client.Namespace
	channel := base.Channel(name, provisonerName)

	channels := client.Eventing.EventingV1alpha1().Channels(namespace)
	// update channel with the new reference
	channel, err := channels.Create(channel)
	if err != nil {
		return err
	}
	client.Cleaner.AddObj(channel)
	return nil
}

// CreateChannels will create a list of Channel Resources in Eventing.
func (client *Client) CreateChannels(names []string, provisionerName string) error {
	for _, name := range names {
		if err := client.CreateChannel(name, provisionerName); err != nil {
			return err
		}
	}
	return nil
}

// CreateSubscription will create a Subscription.
func (client *Client) CreateSubscription(name, channelName string, options ...func(*v1alpha1.Subscription)) error {
	namespace := client.Namespace
	subscription := base.Subscription(name, channelName, options...)

	subscriptions := client.Eventing.EventingV1alpha1().Subscriptions(namespace)
	// update subscription with the new reference
	subscription, err := subscriptions.Create(subscription)
	if err != nil {
		return err
	}
	client.Cleaner.AddObj(subscription)
	return nil
}

// CreateSubscriptions will create a list of Subscriptions.
func (client *Client) CreateSubscriptions(names []string, channelName string, options ...func(*v1alpha1.Subscription)) error {
	for _, name := range names {
		if err := client.CreateSubscription(name, channelName, options...); err != nil {
			return err
		}
	}
	return nil
}

// CreateBroker will create a Broker.
func (client *Client) CreateBroker(name, provisionerName string) error {
	namespace := client.Namespace
	broker := base.Broker(name, provisionerName)

	brokers := client.Eventing.EventingV1alpha1().Brokers(namespace)
	// update broker with the new reference
	broker, err := brokers.Create(broker)
	if err != nil {
		return err
	}
	client.Cleaner.AddObj(broker)
	return nil
}

// CreateBrokers will create a list of Brokers.
func (client *Client) CreateBrokers(names []string, provisionerName string) error {
	for _, name := range names {
		if err := client.CreateBroker(name, provisionerName); err != nil {
			return err
		}
	}
	return nil
}

// CreateTrigger will create a Trigger.
func (client *Client) CreateTrigger(name string, options ...func(*v1alpha1.Trigger)) error {
	namespace := client.Namespace
	trigger := base.Trigger(name, options...)

	triggers := client.Eventing.EventingV1alpha1().Triggers(namespace)
	// update trigger with the new reference
	trigger, err := triggers.Create(trigger)
	if err != nil {
		return err
	}
	client.Cleaner.AddObj(trigger)
	return nil
}

// WithService returns an option that creates a Service binded with the given pod.
func WithService(name string) func(*corev1.Pod, *Client) error {
	return func(pod *corev1.Pod, client *Client) error {
		namespace := pod.Namespace
		svc := base.Service(name, pod.Labels)

		svcs := client.Kube.Kube.CoreV1().Services(namespace)
		if _, err := svcs.Create(svc); err != nil {
			return err
		}
		client.Cleaner.Add(coreAPIGroup, coreAPIVersion, "services", namespace, name)
		return nil
	}
}

// CreatePod will create a Pod.
func (client *Client) CreatePod(pod *corev1.Pod, options ...func(*corev1.Pod, *Client) error) error {
	// set namespace for the pod in case it's empty
	namespace := client.Namespace
	pod.Namespace = namespace
	// apply options on the pod before creation
	for _, option := range options {
		if err := option(pod, client); err != nil {
			return err
		}
	}
	if _, err := client.Kube.CreatePod(pod); err != nil {
		return err
	}
	client.Cleaner.Add(coreAPIGroup, coreAPIVersion, "pods", namespace, pod.Name)
	return nil
}

// CreateServiceAccountAndBinding creates both ServiceAccount and ClusterRoleBinding with default
// cluster-admin role.
func (client *Client) CreateServiceAccountAndBinding(saName, crName string) error {
	namespace := client.Namespace
	sa := base.ServiceAccount(saName, namespace)
	sas := client.Kube.Kube.CoreV1().ServiceAccounts(namespace)
	if _, err := sas.Create(sa); err != nil {
		return err
	}
	client.Cleaner.Add(coreAPIGroup, coreAPIVersion, "serviceaccounts", namespace, saName)

	crb := base.ClusterRoleBinding(saName, crName, namespace)
	crbs := client.Kube.Kube.RbacV1().ClusterRoleBindings()
	if _, err := crbs.Create(crb); err != nil {
		return err
	}
	client.Cleaner.Add(rbacAPIGroup, rbacAPIVersion, "clusterrolebindings", "", crb.GetName())
	return nil
}
