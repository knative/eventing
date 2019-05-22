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
	"fmt"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/test/base"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var eventingAPIGroup = v1alpha1.SchemeGroupVersion.Group
var eventingAPIVersion = v1alpha1.SchemeGroupVersion.Version
var coreAPIGroup = corev1.SchemeGroupVersion.Group
var coreAPIVersion = corev1.SchemeGroupVersion.Version
var rbacAPIGroup = rbacv1.SchemeGroupVersion.Group
var rbacAPIVersion = rbacv1.SchemeGroupVersion.Version

// CreateChannel will create a Channel Resource in Eventing.
func (client *Client) CreateChannel(name, provisonerName string) error {
	namespace := client.Namespace
	channel := base.Channel(name, provisonerName)

	channels := client.Eventing.EventingV1alpha1().Channels(namespace)
	_, err := channels.Create(channel)
	if err != nil {
		return err
	}
	client.Cleaner.Add(eventingAPIGroup, eventingAPIVersion, "channels", namespace, name)
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
	_, err := subscriptions.Create(subscription)
	if err != nil {
		return err
	}
	client.Cleaner.Add(eventingAPIGroup, eventingAPIVersion, "subscriptions", namespace, name)
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
	_, err := brokers.Create(broker)
	if err != nil {
		return err
	}
	client.Cleaner.Add(eventingAPIGroup, eventingAPIVersion, "brokers", namespace, name)
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
	_, err := triggers.Create(trigger)
	if err != nil {
		return err
	}
	client.Cleaner.Add(eventingAPIGroup, eventingAPIVersion, "triggers", namespace, name)
	return nil
}

// WithService returns an option that creates a Service binded with the given pod.
func WithService(name string) func(*corev1.Pod, *Client) error {
	return func(pod *corev1.Pod, client *Client) error {
		namespace := pod.Namespace
		svc := base.Service(name, pod.Labels)

		svcs := client.Kube.Kube.CoreV1().Services(namespace)
		_, err := svcs.Create(svc)
		if err != nil {
			return err
		}
		client.Cleaner.Add(coreAPIGroup, coreAPIVersion, "services", namespace, name)
		return nil
	}
}

// CreatePod will create a Pod.
func (client *Client) CreatePod(pod *corev1.Pod, options ...func(*corev1.Pod, *Client) error) error {
	namespace := client.Namespace
	pod.Namespace = namespace
	_, err := client.Kube.CreatePod(pod)
	if err != nil {
		return err
	}
	client.Cleaner.Add(coreAPIGroup, coreAPIVersion, "pods", namespace, pod.Name)

	for _, option := range options {
		if err := option(pod, client); err != nil {
			return err
		}
	}
	return nil
}

// CreateServiceAccountAndBinding creates both ServiceAccount and ClusterRoleBinding with default
// cluster-admin role.
func (client *Client) CreateServiceAccountAndBinding(saName, crName string) error {
	namespace := client.Namespace
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: namespace,
		},
	}
	err := client.createServiceAccount(sa)
	if err != nil {
		return err
	}
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s-admin", sa.Name, sa.Namespace),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     crName,
			APIGroup: rbacAPIGroup,
		},
	}
	err = client.createClusterRoleBinding(crb)
	if err != nil {
		return err
	}
	return nil
}

// createServiceAccount will create a service account.
func (client *Client) createServiceAccount(sa *corev1.ServiceAccount) error {
	namespace := client.Namespace
	sas := client.Kube.Kube.CoreV1().ServiceAccounts(namespace)
	_, err := sas.Create(sa)
	if err != nil {
		return err
	}
	client.Cleaner.Add(coreAPIGroup, coreAPIVersion, "serviceaccounts", namespace, sa.Name)
	return nil
}

// createClusterRoleBinding will create a service account binding.
func (client *Client) createClusterRoleBinding(crb *rbacv1.ClusterRoleBinding) error {
	clusterRoleBindings := client.Kube.Kube.RbacV1().ClusterRoleBindings()
	_, err := clusterRoleBindings.Create(crb)
	if err != nil {
		return err
	}
	client.Cleaner.Add(rbacAPIGroup, rbacAPIVersion, "clusterrolebindings", "", crb.Name)
	return nil
}
