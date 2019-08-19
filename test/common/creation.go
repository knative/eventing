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

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/test/base"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/pkg/test/helpers"
)

// TODO(Fredy-Z): break this file into multiple files when it grows too large.

var coreAPIGroup = corev1.SchemeGroupVersion.Group
var coreAPIVersion = corev1.SchemeGroupVersion.Version
var rbacAPIGroup = rbacv1.SchemeGroupVersion.Group
var rbacAPIVersion = rbacv1.SchemeGroupVersion.Version

// CreateChannelOrFail will create a Channel Resource in Eventing.
func (client *Client) CreateChannelOrFail(name string, channelTypeMeta *metav1.TypeMeta) {
	namespace := client.Namespace
	metaResource := resources.NewMetaResource(name, namespace, channelTypeMeta)
	gvr, err := base.CreateGenericChannelObject(client.Dynamic, metaResource)
	if err != nil {
		client.T.Fatalf("Failed to create %q %q: %v", channelTypeMeta.Kind, name, err)
	}
	client.Tracker.Add(gvr.Group, gvr.Version, gvr.Resource, namespace, name)
}

// CreateChannelsOrFail will create a list of Channel Resources in Eventing.
func (client *Client) CreateChannelsOrFail(names []string, channelTypeMeta *metav1.TypeMeta) {
	for _, name := range names {
		client.CreateChannelOrFail(name, channelTypeMeta)
	}
}

// CreateSubscriptionOrFail will create a Subscription or fail the test if there is an error.
func (client *Client) CreateSubscriptionOrFail(
	name, channelName string,
	channelTypeMeta *metav1.TypeMeta,
	options ...resources.SubscriptionOption,
) {
	namespace := client.Namespace
	subscription := resources.Subscription(name, channelName, channelTypeMeta, options...)

	subscriptions := client.Eventing.EventingV1alpha1().Subscriptions(namespace)
	// update subscription with the new reference
	subscription, err := subscriptions.Create(subscription)
	if err != nil {
		client.T.Fatalf("Failed to create subscription %q: %v", name, err)
	}
	client.Tracker.AddObj(subscription)
}

// CreateSubscriptionsOrFail will create a list of Subscriptions with the same configuration except the name.
func (client *Client) CreateSubscriptionsOrFail(
	names []string,
	channelName string,
	channelTypeMeta *metav1.TypeMeta,
	options ...resources.SubscriptionOption,
) {
	for _, name := range names {
		client.CreateSubscriptionOrFail(name, channelName, channelTypeMeta, options...)
	}
}

// CreateBrokerOrFail will create a Broker or fail the test if there is an error.
func (client *Client) CreateBrokerOrFail(name string, channelTypeMeta *metav1.TypeMeta) {
	namespace := client.Namespace
	broker := resources.Broker(name, resources.WithChannelTemplateForBroker(*channelTypeMeta))

	brokers := client.Eventing.EventingV1alpha1().Brokers(namespace)
	// update broker with the new reference
	broker, err := brokers.Create(broker)
	if err != nil {
		client.T.Fatalf("Failed to create broker %q: %v", name, err)
	}
	client.Tracker.AddObj(broker)
}

// CreateBrokersOrFail will create a list of Brokers.
func (client *Client) CreateBrokersOrFail(names []string, channelTypeMeta *metav1.TypeMeta) {
	for _, name := range names {
		client.CreateBrokerOrFail(name, channelTypeMeta)
	}
}

// CreateTriggerOrFail will create a Trigger or fail the test if there is an error.
func (client *Client) CreateTriggerOrFail(name string, options ...resources.TriggerOption) {
	namespace := client.Namespace
	trigger := resources.Trigger(name, options...)

	triggers := client.Eventing.EventingV1alpha1().Triggers(namespace)
	// update trigger with the new reference
	trigger, err := triggers.Create(trigger)
	if err != nil {
		client.T.Fatalf("Failed to create trigger %q: %v", name, err)
	}
	client.Tracker.AddObj(trigger)
}

// CreateSequenceOrFail will create a Sequence or fail the test if there is an error.
func (client *Client) CreateSequenceOrFail(sequence *messagingv1alpha1.Sequence) {
	sequences := client.Eventing.MessagingV1alpha1().Sequences(client.Namespace)
	_, err := sequences.Create(sequence)
	if err != nil {
		client.T.Fatalf("Failed to create sequence %q: %v", sequence.Name, err)
	}
	client.Tracker.AddObj(sequence)
}

// CreateChoiceOrFail will create a Choice or fail the test if there is an error.
func (client *Client) CreateChoiceOrFail(choice *messagingv1alpha1.Choice) {
	choices := client.Eventing.MessagingV1alpha1().Choices(client.Namespace)
	_, err := choices.Create(choice)
	if err != nil {
		client.T.Fatalf("Failed to create choice %q: %v", choice.Name, err)
	}
	client.Tracker.AddObj(choice)
}

// CreateCronJobSourceOrFail will create a CronJobSource or fail the test if there is an error.
func (client *Client) CreateCronJobSourceOrFail(cronJobSource *sourcesv1alpha1.CronJobSource) {
	cronJobSourceInterface := client.Eventing.SourcesV1alpha1().CronJobSources(client.Namespace)
	_, err := cronJobSourceInterface.Create(cronJobSource)
	if err != nil {
		client.T.Fatalf("Failed to create cronjobsource %q: %v", cronJobSource.Name, err)
	}
	client.Tracker.AddObj(cronJobSource)
}

// CreateContainerSourceOrFail will create a ContainerSource or fail the test if there is an error.
func (client *Client) CreateContainerSourceOrFail(containerSource *sourcesv1alpha1.ContainerSource) {
	containerSourceInterface := client.Eventing.SourcesV1alpha1().ContainerSources(client.Namespace)
	_, err := containerSourceInterface.Create(containerSource)
	if err != nil {
		client.T.Fatalf("Failed to create containersource %q: %v", containerSource.Name, err)
	}
	client.Tracker.AddObj(containerSource)
}

// CreateApiServerSourceOrFail will create an ApiServerSource
func (client *Client) CreateApiServerSourceOrFail(apiServerSource *sourcesv1alpha1.ApiServerSource) {
	apiServerInterface := client.Eventing.SourcesV1alpha1().ApiServerSources(client.Namespace)
	_, err := apiServerInterface.Create(apiServerSource)
	if err != nil {
		client.T.Fatalf("Failed to create apiserversource %q: %v", apiServerSource.Name, err)
	}
	client.Tracker.AddObj(apiServerSource)
}

// WithService returns an option that creates a Service binded with the given pod.
func WithService(name string) func(*corev1.Pod, *Client) error {
	return func(pod *corev1.Pod, client *Client) error {
		namespace := pod.Namespace
		svc := resources.Service(name, pod.Labels)

		svcs := client.Kube.Kube.CoreV1().Services(namespace)
		if _, err := svcs.Create(svc); err != nil {
			return err
		}
		client.Tracker.Add(coreAPIGroup, coreAPIVersion, "services", namespace, name)
		return nil
	}
}

// CreatePodOrFail will create a Pod or fail the test if there is an error.
func (client *Client) CreatePodOrFail(pod *corev1.Pod, options ...func(*corev1.Pod, *Client) error) {
	// set namespace for the pod in case it's empty
	namespace := client.Namespace
	pod.Namespace = namespace
	// apply options on the pod before creation
	for _, option := range options {
		if err := option(pod, client); err != nil {
			client.T.Fatalf("Failed to configure pod %q: %v", pod.Name, err)
		}
	}
	if _, err := client.Kube.CreatePod(pod); err != nil {
		client.T.Fatalf("Failed to create pod %q: %v", pod.Name, err)
	}
	client.Tracker.Add(coreAPIGroup, coreAPIVersion, "pods", namespace, pod.Name)
}

// CreateServiceAccountOrFail will create a ServiceAccount or fail the test if there is an error.
func (client *Client) CreateServiceAccountOrFail(saName string) {
	namespace := client.Namespace
	sa := resources.ServiceAccount(saName, namespace)
	sas := client.Kube.Kube.CoreV1().ServiceAccounts(namespace)
	if _, err := sas.Create(sa); err != nil {
		client.T.Fatalf("Failed to create service account %q: %v", saName, err)
	}
	client.Tracker.Add(coreAPIGroup, coreAPIVersion, "serviceaccounts", namespace, saName)
}

// CreateClusterRoleOrFail creates the given ClusterRole or fail the test if there is an error.
func (client *Client) CreateClusterRoleOrFail(cr *rbacv1.ClusterRole) {
	crs := client.Kube.Kube.RbacV1().ClusterRoles()
	if _, err := crs.Create(cr); err != nil {
		client.T.Fatalf("Failed to create cluster role %q: %v", cr.Name, err)
	}
	client.Tracker.Add(rbacAPIGroup, rbacAPIVersion, "clusterroles", "", cr.Name)
}

// CreateRoleBindingOrFail will create a RoleBinding or fail the test if there is an error.
func (client *Client) CreateRoleBindingOrFail(saName, crName, rbName, rbNamespace string) {
	saNamespace := client.Namespace
	rb := resources.RoleBinding(saName, saNamespace, crName, rbName, rbNamespace)
	rbs := client.Kube.Kube.RbacV1().RoleBindings(rbNamespace)
	if _, err := rbs.Create(rb); err != nil {
		client.T.Fatalf("Failed to create role binding %q: %v", rbName, err)
	}
	client.Tracker.Add(rbacAPIGroup, rbacAPIVersion, "rolebindings", rbNamespace, rb.GetName())
}

// CreateClusterRoleBindingOrFail will create a ClusterRoleBinding or fail the test if there is an error.
func (client *Client) CreateClusterRoleBindingOrFail(saName, crName, crbName string) {
	saNamespace := client.Namespace
	crb := resources.ClusterRoleBinding(saName, saNamespace, crName, crbName)
	crbs := client.Kube.Kube.RbacV1().ClusterRoleBindings()
	if _, err := crbs.Create(crb); err != nil {
		client.T.Fatalf("Failed to create cluster role binding %q: %v", crbName, err)
	}
	client.Tracker.Add(rbacAPIGroup, rbacAPIVersion, "clusterrolebindings", "", crb.GetName())
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
func (client *Client) CreateRBACResourcesForBrokers() {
	client.CreateServiceAccountOrFail(saIngressName)
	client.CreateServiceAccountOrFail(saFilterName)
	// The two RoleBindings are required for running Brokers correctly.
	client.CreateRoleBindingOrFail(
		saIngressName,
		crIngressName,
		fmt.Sprintf("%s-%s", saIngressName, crIngressName),
		client.Namespace,
	)
	client.CreateRoleBindingOrFail(
		saFilterName,
		crFilterName,
		fmt.Sprintf("%s-%s", saFilterName, crFilterName),
		client.Namespace,
	)
	// The two RoleBindings are required for access to shared configmaps for logging,
	// tracing, and metrics configuration.
	client.CreateRoleBindingOrFail(
		saIngressName,
		crConfigReaderName,
		fmt.Sprintf("%s-%s-%s", saIngressName, helpers.MakeK8sNamePrefix(client.Namespace), crConfigReaderName),
		resources.SystemNamespace,
	)
	client.CreateRoleBindingOrFail(
		saFilterName,
		crConfigReaderName,
		fmt.Sprintf("%s-%s-%s", saFilterName, helpers.MakeK8sNamePrefix(client.Namespace), crConfigReaderName),
		resources.SystemNamespace,
	)
}
