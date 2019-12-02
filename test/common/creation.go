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

	appsv1 "k8s.io/api/apps/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	flowsv1alpha1 "knative.dev/eventing/pkg/apis/flows/v1alpha1"
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

// CreateChannelOrFail will create a typed Channel Resource in Eventing or fail the test if there is an error.
func (client *Client) CreateChannelOrFail(name string, channelTypeMeta *metav1.TypeMeta) {
	namespace := client.Namespace
	metaResource := resources.NewMetaResource(name, namespace, channelTypeMeta)
	gvr, err := base.CreateGenericChannelObject(client.Dynamic, metaResource)
	if err != nil {
		client.T.Fatalf("Failed to create %q %q: %v", channelTypeMeta.Kind, name, err)
	}
	client.Tracker.Add(gvr.Group, gvr.Version, gvr.Resource, namespace, name)
}

// CreateChannelsOrFail will create a list of typed Channel Resources in Eventing or fail the test if there is an error.
func (client *Client) CreateChannelsOrFail(names []string, channelTypeMeta *metav1.TypeMeta) {
	for _, name := range names {
		client.CreateChannelOrFail(name, channelTypeMeta)
	}
}

// CreateChannelWithDefaultOrFail will create a default Channel Resource in Eventing or fail the test if there is an error.
func (client *Client) CreateChannelWithDefaultOrFail(channel *messagingv1alpha1.Channel) {
	channels := client.Eventing.MessagingV1alpha1().Channels(client.Namespace)
	_, err := channels.Create(channel)
	if err != nil {
		client.T.Fatalf("Failed to create channel %q: %v", channel.Name, err)
	}
	client.Tracker.AddObj(channel)
}

// CreateSubscriptionOrFail will create a Subscription or fail the test if there is an error.
func (client *Client) CreateSubscriptionOrFail(
	name, channelName string,
	channelTypeMeta *metav1.TypeMeta,
	options ...resources.SubscriptionOption,
) {
	namespace := client.Namespace
	subscription := resources.Subscription(name, channelName, channelTypeMeta, options...)

	subscriptions := client.Eventing.MessagingV1alpha1().Subscriptions(namespace)
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
func (client *Client) CreateBrokerOrFail(name string, channelTypeMeta *metav1.TypeMeta) *v1alpha1.Broker {
	namespace := client.Namespace
	broker := resources.Broker(name, resources.WithChannelTemplateForBroker(*channelTypeMeta))

	brokers := client.Eventing.EventingV1alpha1().Brokers(namespace)
	// update broker with the new reference
	broker, err := brokers.Create(broker)
	if err != nil {
		client.T.Fatalf("Failed to create broker %q: %v", name, err)
	}
	client.Tracker.AddObj(broker)
	return broker
}

// CreateBrokersOrFail will create a list of Brokers.
func (client *Client) CreateBrokersOrFail(names []string, channelTypeMeta *metav1.TypeMeta) {
	for _, name := range names {
		client.CreateBrokerOrFail(name, channelTypeMeta)
	}
}

// CreateTriggerOrFail will create a Trigger or fail the test if there is an error.
func (client *Client) CreateTriggerOrFail(name string, options ...resources.TriggerOption) *v1alpha1.Trigger {
	namespace := client.Namespace
	trigger := resources.Trigger(name, options...)

	triggers := client.Eventing.EventingV1alpha1().Triggers(namespace)
	// update trigger with the new reference
	trigger, err := triggers.Create(trigger)
	if err != nil {
		client.T.Fatalf("Failed to create trigger %q: %v", name, err)
	}
	client.Tracker.AddObj(trigger)
	return trigger
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

// CreateFlowsSequenceOrFail will create a Sequence (in flows.knative.dev api group) or
// fail the test if there is an error.
func (client *Client) CreateFlowsSequenceOrFail(sequence *flowsv1alpha1.Sequence) {
	sequences := client.Eventing.FlowsV1alpha1().Sequences(client.Namespace)
	_, err := sequences.Create(sequence)
	if err != nil {
		client.T.Fatalf("Failed to create sequence %q: %v", sequence.Name, err)
	}
	client.Tracker.AddObj(sequence)
}

// CreateParallelOrFail will create a Parallel or fail the test if there is an error.
func (client *Client) CreateParallelOrFail(parallel *messagingv1alpha1.Parallel) {
	parallels := client.Eventing.MessagingV1alpha1().Parallels(client.Namespace)
	_, err := parallels.Create(parallel)
	if err != nil {
		client.T.Fatalf("Failed to create parallel %q: %v", parallel.Name, err)
	}
	client.Tracker.AddObj(parallel)
}

// CreateFlowsParallelOrFail will create a Parallel (in flows.knative.dev api group) or
// fail the test if there is an error.
func (client *Client) CreateFlowsParallelOrFail(parallel *flowsv1alpha1.Parallel) {
	parallels := client.Eventing.FlowsV1alpha1().Parallels(client.Namespace)
	_, err := parallels.Create(parallel)
	if err != nil {
		client.T.Fatalf("Failed to create flows parallel %q: %v", parallel.Name, err)
	}
	client.Tracker.AddObj(parallel)
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

// CreateSinkBindingOrFail will create a SinkBinding or fail the test if there is an error.
func (client *Client) CreateSinkBindingOrFail(containerSource *sourcesv1alpha1.SinkBinding) {
	containerSourceInterface := client.Eventing.SourcesV1alpha1().SinkBindings(client.Namespace)
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

func (client *Client) CreateServiceOrFail(svc *corev1.Service) *corev1.Service {
	namespace := client.Namespace
	if newSvc, err := client.Kube.Kube.CoreV1().Services(namespace).Create(svc); err != nil {
		client.T.Fatalf("Failed to create service %q: %v", svc.Name, err)
		return nil
	} else {
		client.Tracker.Add(coreAPIGroup, coreAPIVersion, "services", namespace, svc.Name)
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
	client.podsCreated = append(client.podsCreated, pod.Name)
}

// CreateDeploymentOrFail will create a Deployment or fail the test if there is an error.
func (client *Client) CreateDeploymentOrFail(deploy *appsv1.Deployment, options ...func(*appsv1.Deployment, *Client) error) {
	// set namespace for the deploy in case it's empty
	namespace := client.Namespace
	deploy.Namespace = namespace
	// apply options on the deploy before creation
	for _, option := range options {
		if err := option(deploy, client); err != nil {
			client.T.Fatalf("Failed to configure deploy %q: %v", deploy.Name, err)
		}
	}
	if _, err := client.Kube.Kube.AppsV1().Deployments(deploy.Namespace).Create(deploy); err != nil {
		client.T.Fatalf("Failed to create deploy %q: %v", deploy.Name, err)
	}
	client.Tracker.Add("apps", "v1", "deployments", namespace, deploy.Name)
}

// CreateCronJobOrFail will create a CronJob or fail the test if there is an error.
func (client *Client) CreateCronJobOrFail(cronjob *batchv1beta1.CronJob, options ...func(*batchv1beta1.CronJob, *Client) error) {
	// set namespace for the cronjob in case it's empty
	namespace := client.Namespace
	cronjob.Namespace = namespace
	// apply options on the cronjob before creation
	for _, option := range options {
		if err := option(cronjob, client); err != nil {
			client.T.Fatalf("Failed to configure cronjob %q: %v", cronjob.Name, err)
		}
	}
	if _, err := client.Kube.Kube.BatchV1beta1().CronJobs(cronjob.Namespace).Create(cronjob); err != nil {
		client.T.Fatalf("Failed to create cronjob %q: %v", cronjob.Name, err)
	}
	client.Tracker.Add("batch", "v1beta1", "cronjobs", namespace, cronjob.Name)
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

	// If the "default" Namespace has a secret called
	// "kn-eventing-test-pull-secret" then use that as the ImagePullSecret
	// on the new ServiceAccount we just created.
	// This is needed for cases where the images are in a private registry.
	_, err := CopySecret(client, "default", TestPullSecretName, namespace, saName)
	if err != nil && !errors.IsNotFound(err) {
		client.T.Fatalf("Error copying the secret: %s", err)
	}
}

// CreateClusterRoleOrFail creates the given ClusterRole or fail the test if there is an error.
func (client *Client) CreateClusterRoleOrFail(cr *rbacv1.ClusterRole) {
	crs := client.Kube.Kube.RbacV1().ClusterRoles()
	if _, err := crs.Create(cr); err != nil && !errors.IsAlreadyExists(err) {
		client.T.Fatalf("Failed to create cluster role %q: %v", cr.Name, err)
	}
	client.Tracker.Add(rbacAPIGroup, rbacAPIVersion, "clusterroles", "", cr.Name)
}

// CreateRoleOrFail creates the given Role in the Client namespace or fail the test if there is an error.
func (client *Client) CreateRoleOrFail(r *rbacv1.Role) {
	namespace := client.Namespace
	rs := client.Kube.Kube.RbacV1().Roles(namespace)
	if _, err := rs.Create(r); err != nil && !errors.IsAlreadyExists(err) {
		client.T.Fatalf("Failed to create cluster role %q: %v", r.Name, err)
	}
	client.Tracker.Add(rbacAPIGroup, rbacAPIVersion, "roles", namespace, r.Name)
}

const (
	ClusterRoleKind = "ClusterRole"
	RoleKind        = "Role"
)

// CreateRoleBindingOrFail will create a RoleBinding or fail the test if there is an error.
func (client *Client) CreateRoleBindingOrFail(saName, rKind, rName, rbName, rbNamespace string) {
	saNamespace := client.Namespace
	rb := resources.RoleBinding(saName, saNamespace, rKind, rName, rbName, rbNamespace)
	rbs := client.Kube.Kube.RbacV1().RoleBindings(rbNamespace)

	if _, err := rbs.Create(rb); err != nil && !errors.IsAlreadyExists(err) {
		client.T.Fatalf("Failed to create role binding %q: %v", rbName, err)
	}
	client.Tracker.Add(rbacAPIGroup, rbacAPIVersion, "rolebindings", rbNamespace, rb.GetName())
}

// CreateClusterRoleBindingOrFail will create a ClusterRoleBinding or fail the test if there is an error.
func (client *Client) CreateClusterRoleBindingOrFail(saName, crName, crbName string) {
	saNamespace := client.Namespace
	crb := resources.ClusterRoleBinding(saName, saNamespace, crName, crbName)
	crbs := client.Kube.Kube.RbacV1().ClusterRoleBindings()
	if _, err := crbs.Create(crb); err != nil && !errors.IsAlreadyExists(err) {
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
		ClusterRoleKind,
		crIngressName,
		fmt.Sprintf("%s-%s", saIngressName, crIngressName),
		client.Namespace,
	)
	client.CreateRoleBindingOrFail(
		saFilterName,
		ClusterRoleKind,
		crFilterName,
		fmt.Sprintf("%s-%s", saFilterName, crFilterName),
		client.Namespace,
	)
	// The two RoleBindings are required for access to shared configmaps for logging,
	// tracing, and metrics configuration.
	client.CreateRoleBindingOrFail(
		saIngressName,
		ClusterRoleKind,
		crConfigReaderName,
		fmt.Sprintf("%s-%s-%s", saIngressName, helpers.MakeK8sNamePrefix(client.Namespace), crConfigReaderName),
		resources.SystemNamespace,
	)
	client.CreateRoleBindingOrFail(
		saFilterName,
		ClusterRoleKind,
		crConfigReaderName,
		fmt.Sprintf("%s-%s-%s", saFilterName, helpers.MakeK8sNamePrefix(client.Namespace), crConfigReaderName),
		resources.SystemNamespace,
	)
}
