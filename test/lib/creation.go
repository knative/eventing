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

	appsv1 "k8s.io/api/apps/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configsv1alpha1 "knative.dev/eventing/pkg/apis/configs/v1alpha1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	flowsv1alpha1 "knative.dev/eventing/pkg/apis/flows/v1alpha1"
	legacysourcesv1alpha1 "knative.dev/eventing/pkg/apis/legacysources/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
)

// TODO(chizhg): break this file into multiple files when it grows too large.

var coreAPIGroup = corev1.SchemeGroupVersion.Group
var coreAPIVersion = corev1.SchemeGroupVersion.Version
var rbacAPIGroup = rbacv1.SchemeGroupVersion.Group
var rbacAPIVersion = rbacv1.SchemeGroupVersion.Version

// CreateChannelOrFail will create a typed Channel Resource in Eventing or fail the test if there is an error.
func (client *Client) CreateChannelOrFail(name string, channelTypeMeta *metav1.TypeMeta) {
	client.T.Logf("Creating channel %+v-%s", channelTypeMeta, name)
	namespace := client.Namespace
	metaResource := resources.NewMetaResource(name, namespace, channelTypeMeta)
	gvr, err := duck.CreateGenericChannelObject(client.Dynamic, metaResource)
	if err != nil {
		client.T.Fatalf("Failed to create %q %q: %v", channelTypeMeta.Kind, name, err)
	}
	client.Tracker.Add(gvr.Group, gvr.Version, gvr.Resource, namespace, name)
}

// CreateChannelsOrFail will create a list of typed Channel Resources in Eventing or fail the test if there is an error.
func (client *Client) CreateChannelsOrFail(names []string, channelTypeMeta *metav1.TypeMeta) {
	client.T.Logf("Creating channels %+v-%v", channelTypeMeta, names)
	for _, name := range names {
		client.CreateChannelOrFail(name, channelTypeMeta)
	}
}

// CreateChannelWithDefaultOrFail will create a default Channel Resource in Eventing or fail the test if there is an error.
func (client *Client) CreateChannelWithDefaultOrFail(channel *messagingv1alpha1.Channel) {
	client.T.Logf("Creating default channel %+v", channel)
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
	client.T.Logf("Creating subscription %s for channel %+v-%s", name, channelTypeMeta, channelName)
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
	client.T.Logf("Creating subscriptions %v for channel %+v-%s", names, channelTypeMeta, channelName)
	for _, name := range names {
		client.CreateSubscriptionOrFail(name, channelName, channelTypeMeta, options...)
	}
}

func (client *Client) CreateConfigMapPropagationOrFail(name string) *configsv1alpha1.ConfigMapPropagation {
	client.T.Logf("Creating configMapPropagation %s", name)
	namespace := client.Namespace
	configMapPropagation := resources.ConfigMapPropagation(name, namespace)
	configMapPropagations := client.Eventing.ConfigsV1alpha1().ConfigMapPropagations(namespace)
	configMapPropagation, err := configMapPropagations.Create(configMapPropagation)
	if err != nil {
		client.T.Fatalf("Failed to create configMapPropagation %q: %v", name, err)
	}
	client.Tracker.AddObj(configMapPropagation)
	return configMapPropagation
}

// CreateConfigMapOrFail will create a configmap or fail the test if there is an error.
func (client *Client) CreateConfigMapOrFail(name, namespace string, data map[string]string) *corev1.ConfigMap {
	client.T.Logf("Creating configmap %s", name)
	configMap, err := client.Kube.Kube.CoreV1().ConfigMaps(namespace).Create(resources.ConfigMap(name, data))
	if err != nil {
		client.T.Fatalf("Failed to create configmap %s: %v", name, err)
	}
	client.Tracker.Add(coreAPIGroup, coreAPIVersion, "configmaps", namespace, name)
	return configMap
}

// CreateBrokerOrFail will create a Broker or fail the test if there is an error.
func (client *Client) CreateBrokerOrFail(name string, options ...resources.BrokerOption) *v1alpha1.Broker {
	namespace := client.Namespace
	broker := resources.Broker(name, options...)
	brokers := client.Eventing.EventingV1alpha1().Brokers(namespace)
	client.T.Logf("Creating broker %s", name)
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
	client.T.Logf("Creating brokers %v", names)
	for _, name := range names {
		client.CreateBrokerOrFail(name, resources.WithChannelTemplateForBroker(channelTypeMeta))
	}
}

// CreateTriggerOrFail will create a Trigger or fail the test if there is an error.
func (client *Client) CreateTriggerOrFail(name string, options ...resources.TriggerOption) *v1alpha1.Trigger {
	namespace := client.Namespace
	trigger := resources.Trigger(name, options...)
	triggers := client.Eventing.EventingV1alpha1().Triggers(namespace)
	client.T.Logf("Creating trigger %s", name)
	// update trigger with the new reference
	trigger, err := triggers.Create(trigger)
	if err != nil {
		client.T.Fatalf("Failed to create trigger %q: %v", name, err)
	}
	client.Tracker.AddObj(trigger)
	return trigger
}

// CreateFlowsSequenceOrFail will create a Sequence (in flows.knative.dev api group) or
// fail the test if there is an error.
func (client *Client) CreateFlowsSequenceOrFail(sequence *flowsv1alpha1.Sequence) {
	client.T.Logf("Creating flows sequence %+v", sequence)
	sequences := client.Eventing.FlowsV1alpha1().Sequences(client.Namespace)
	_, err := sequences.Create(sequence)
	if err != nil {
		client.T.Fatalf("Failed to create flows sequence %q: %v", sequence.Name, err)
	}
	client.Tracker.AddObj(sequence)
}

// CreateFlowsParallelOrFail will create a Parallel (in flows.knative.dev api group) or
// fail the test if there is an error.
func (client *Client) CreateFlowsParallelOrFail(parallel *flowsv1alpha1.Parallel) {
	client.T.Logf("Creating flows parallel %+v", parallel)
	parallels := client.Eventing.FlowsV1alpha1().Parallels(client.Namespace)
	_, err := parallels.Create(parallel)
	if err != nil {
		client.T.Fatalf("Failed to create flows parallel %q: %v", parallel.Name, err)
	}
	client.Tracker.AddObj(parallel)
}

// CreateLegacyCronJobSourceOrFail will create a CronJobSource or fail the test if there is an error.
func (client *Client) CreateLegacyCronJobSourceOrFail(cronJobSource *legacysourcesv1alpha1.CronJobSource) {
	client.T.Logf("Creating legacy cronjobsource %+v", cronJobSource)
	cronJobSourceInterface := client.Legacy.SourcesV1alpha1().CronJobSources(client.Namespace)
	_, err := cronJobSourceInterface.Create(cronJobSource)
	if err != nil {
		client.T.Fatalf("Failed to create cronjobsource %q: %v", cronJobSource.Name, err)
	}
	client.Tracker.AddObj(cronJobSource)
}

// CreateLegacyContainerSourceOrFail will create a ContainerSource or fail the test if there is an error.
func (client *Client) CreateLegacyContainerSourceOrFail(containerSource *legacysourcesv1alpha1.ContainerSource) {
	client.T.Logf("Creating legacy containersource %+v", containerSource)
	containerSourceInterface := client.Legacy.SourcesV1alpha1().ContainerSources(client.Namespace)
	_, err := containerSourceInterface.Create(containerSource)
	if err != nil {
		client.T.Fatalf("Failed to create containersource %q: %v", containerSource.Name, err)
	}
	client.Tracker.AddObj(containerSource)
}

// CreateSinkBindingOrFail will create a SinkBinding or fail the test if there is an error.
func (client *Client) CreateSinkBindingOrFail(sb *sourcesv1alpha1.SinkBinding) {
	client.T.Logf("Creating sinkbinding %+v", sb)
	sbInterface := client.Eventing.SourcesV1alpha1().SinkBindings(client.Namespace)
	_, err := sbInterface.Create(sb)
	if err != nil {
		client.T.Fatalf("Failed to create sinkbinding %q: %v", sb.Name, err)
	}
	client.Tracker.AddObj(sb)
}

// CreateLegacySinkBindingOrFail will create a SinkBinding or fail the test if there is an error.
func (client *Client) CreateLegacySinkBindingOrFail(sb *legacysourcesv1alpha1.SinkBinding) {
	client.T.Logf("Creating sinkbinding %+v", sb)
	sbInterface := client.Legacy.SourcesV1alpha1().SinkBindings(client.Namespace)
	_, err := sbInterface.Create(sb)
	if err != nil {
		client.T.Fatalf("Failed to create sinkbinding %q: %v", sb.Name, err)
	}
	client.Tracker.AddObj(sb)
}

// CreateApiServerSourceOrFail will create an ApiServerSource
func (client *Client) CreateApiServerSourceOrFail(apiServerSource *sourcesv1alpha1.ApiServerSource) {
	client.T.Logf("Creating apiserversource %+v", apiServerSource)
	apiServerInterface := client.Eventing.SourcesV1alpha1().ApiServerSources(client.Namespace)
	_, err := apiServerInterface.Create(apiServerSource)
	if err != nil {
		client.T.Fatalf("Failed to create apiserversource %q: %v", apiServerSource.Name, err)
	}
	client.Tracker.AddObj(apiServerSource)
}

// CreatePingSourceOrFail will create an PingSource
func (client *Client) CreatePingSourceOrFail(pingSource *sourcesv1alpha1.PingSource) {
	client.T.Logf("Creating pingsource %+v", pingSource)
	pingInterface := client.Eventing.SourcesV1alpha1().PingSources(client.Namespace)
	_, err := pingInterface.Create(pingSource)
	if err != nil {
		client.T.Fatalf("Failed to create pingsource %q: %v", pingSource.Name, err)
	}
	client.Tracker.AddObj(pingSource)
}

// CreateLegacyApiServerSourceOrFail will create an ApiServerSource
func (client *Client) CreateLegacyApiServerSourceOrFail(apiServerSource *legacysourcesv1alpha1.ApiServerSource) {
	client.T.Logf("Creating apiserversource %+v", apiServerSource)
	apiServerInterface := client.Legacy.SourcesV1alpha1().ApiServerSources(client.Namespace)
	_, err := apiServerInterface.Create(apiServerSource)
	if err != nil {
		client.T.Fatalf("Failed to create apiserversource %q: %v", apiServerSource.Name, err)
	}
	client.Tracker.AddObj(apiServerSource)
}

func (client *Client) CreateServiceOrFail(svc *corev1.Service) *corev1.Service {
	client.T.Logf("Creating service %+v", svc)
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
	client.T.Logf("Creating pod %+v", pod)
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
	client.T.Logf("Creating deployment %+v", deploy)
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
	client.T.Logf("Creating cronjob %+v", cronjob)
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
	client.T.Logf("Creating service account %+v", sa)
	if _, err := sas.Create(sa); err != nil {
		client.T.Fatalf("Failed to create service account %q: %v", saName, err)
	}
	client.Tracker.Add(coreAPIGroup, coreAPIVersion, "serviceaccounts", namespace, saName)

	// If the "default" Namespace has a secret called
	// "kn-eventing-test-pull-secret" then use that as the ImagePullSecret
	// on the new ServiceAccount we just created.
	// This is needed for cases where the images are in a private registry.
	_, err := utils.CopySecret(client.Kube.Kube.CoreV1(), "default", TestPullSecretName, namespace, saName)
	if err != nil && !errors.IsNotFound(err) {
		client.T.Fatalf("Error copying the secret: %s", err)
	}
}

// CreateClusterRoleOrFail creates the given ClusterRole or fail the test if there is an error.
func (client *Client) CreateClusterRoleOrFail(cr *rbacv1.ClusterRole) {
	client.T.Logf("Creating cluster role %+v", cr)
	crs := client.Kube.Kube.RbacV1().ClusterRoles()
	if _, err := crs.Create(cr); err != nil && !errors.IsAlreadyExists(err) {
		client.T.Fatalf("Failed to create cluster role %q: %v", cr.Name, err)
	}
	client.Tracker.Add(rbacAPIGroup, rbacAPIVersion, "clusterroles", "", cr.Name)
}

// CreateRoleOrFail creates the given Role in the Client namespace or fail the test if there is an error.
func (client *Client) CreateRoleOrFail(r *rbacv1.Role) {
	client.T.Logf("Creating role %+v", r)
	namespace := client.Namespace
	rs := client.Kube.Kube.RbacV1().Roles(namespace)
	if _, err := rs.Create(r); err != nil && !errors.IsAlreadyExists(err) {
		client.T.Fatalf("Failed to create role %q: %v", r.Name, err)
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

	client.T.Logf("Creating role binding %+v", rb)
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

	client.T.Logf("Creating cluster role binding %+v", crb)
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
}
