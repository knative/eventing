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

	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"

	appsv1 "k8s.io/api/apps/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configsv1alpha1 "knative.dev/eventing/pkg/apis/configs/v1alpha1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	flowsv1alpha1 "knative.dev/eventing/pkg/apis/flows/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// TODO(chizhg): break this file into multiple files when it grows too large.

var coreAPIGroup = corev1.SchemeGroupVersion.Group
var coreAPIVersion = corev1.SchemeGroupVersion.Version
var rbacAPIGroup = rbacv1.SchemeGroupVersion.Group
var rbacAPIVersion = rbacv1.SchemeGroupVersion.Version

// CreateChannelOrFail will create a typed Channel Resource in Eventing or fail the test if there is an error.
func (c *Client) CreateChannelOrFail(name string, channelTypeMeta *metav1.TypeMeta) {
	c.T.Logf("Creating channel %+v-%s", channelTypeMeta, name)
	namespace := c.Namespace
	metaResource := resources.NewMetaResource(name, namespace, channelTypeMeta)
	gvr, err := duck.CreateGenericChannelObject(c.Dynamic, metaResource)
	if err != nil {
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
func (c *Client) CreateChannelWithDefaultOrFail(channel *messagingv1alpha1.Channel) {
	c.T.Logf("Creating default channel %+v", channel)
	channels := c.Eventing.MessagingV1alpha1().Channels(c.Namespace)
	_, err := channels.Create(channel)
	if err != nil {
		c.T.Fatalf("Failed to create channel %q: %v", channel.Name, err)
	}
	c.Tracker.AddObj(channel)
}

// CreateSubscriptionOrFail will create a Subscription or fail the test if there is an error.
func (c *Client) CreateSubscriptionOrFail(
	name, channelName string,
	channelTypeMeta *metav1.TypeMeta,
	options ...resources.SubscriptionOption,
) *messagingv1alpha1.Subscription {
	namespace := c.Namespace
	subscription := resources.Subscription(name, channelName, channelTypeMeta, options...)
	subscriptions := c.Eventing.MessagingV1alpha1().Subscriptions(namespace)
	c.T.Logf("Creating subscription %s for channel %+v-%s", name, channelTypeMeta, channelName)
	// update subscription with the new reference
	subscription, err := subscriptions.Create(subscription)
	if err != nil {
		c.T.Fatalf("Failed to create subscription %q: %v", name, err)
	}
	c.Tracker.AddObj(subscription)
	return subscription
}

// CreateSubscriptionOrFailV1Beta1 will create a Subscription or fail the test if there is an error.
func (c *Client) CreateSubscriptionOrFailV1Beta1(
	name, channelName string,
	channelTypeMeta *metav1.TypeMeta,
	options ...resources.SubscriptionOptionV1Beta1,
) {
	namespace := c.Namespace
	subscription := resources.SubscriptionV1Beta1(name, channelName, channelTypeMeta, options...)
	subscriptions := c.Eventing.MessagingV1beta1().Subscriptions(namespace)
	c.T.Logf("Creating v1beta1 subscription %s for channel %+v-%s", name, channelTypeMeta, channelName)
	// update subscription with the new reference
	subscription, err := subscriptions.Create(subscription)
	if err != nil {
		c.T.Fatalf("Failed to create subscription %q: %v", name, err)
	}
	c.Tracker.AddObj(subscription)
}

// CreateSubscriptionsOrFail will create a list of Subscriptions with the same configuration except the name.
func (c *Client) CreateSubscriptionsOrFail(
	names []string,
	channelName string,
	channelTypeMeta *metav1.TypeMeta,
	options ...resources.SubscriptionOption,
) {
	c.T.Logf("Creating subscriptions %v for channel %+v-%s", names, channelTypeMeta, channelName)
	for _, name := range names {
		c.CreateSubscriptionOrFail(name, channelName, channelTypeMeta, options...)
	}
}

func (c *Client) CreateConfigMapPropagationOrFail(name string) *configsv1alpha1.ConfigMapPropagation {
	c.T.Logf("Creating configMapPropagation %s", name)
	namespace := c.Namespace
	configMapPropagation := resources.ConfigMapPropagation(name, namespace)
	configMapPropagations := c.Eventing.ConfigsV1alpha1().ConfigMapPropagations(namespace)
	configMapPropagation, err := configMapPropagations.Create(configMapPropagation)
	if err != nil {
		c.T.Fatalf("Failed to create configMapPropagation %q: %v", name, err)
	}
	c.Tracker.AddObj(configMapPropagation)
	return configMapPropagation
}

// CreateConfigMapOrFail will create a configmap or fail the test if there is an error.
func (c *Client) CreateConfigMapOrFail(name, namespace string, data map[string]string) *corev1.ConfigMap {
	c.T.Logf("Creating configmap %s", name)
	configMap, err := c.Kube.Kube.CoreV1().ConfigMaps(namespace).Create(resources.ConfigMap(name, data))
	if err != nil {
		c.T.Fatalf("Failed to create configmap %s: %v", name, err)
	}
	c.Tracker.Add(coreAPIGroup, coreAPIVersion, "configmaps", namespace, name)
	return configMap
}

// CreateBrokerOrFail will create a Broker or fail the test if there is an error.
func (c *Client) CreateBrokerOrFail(name string, options ...resources.BrokerOption) *v1alpha1.Broker {
	namespace := c.Namespace
	broker := resources.Broker(name, options...)
	brokers := c.Eventing.EventingV1alpha1().Brokers(namespace)
	c.T.Logf("Creating broker %s", name)
	// update broker with the new reference
	broker, err := brokers.Create(broker)
	if err != nil {
		c.T.Fatalf("Failed to create broker %q: %v", name, err)
	}
	c.Tracker.AddObj(broker)
	return broker
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

// CreateBrokerOrFail will create a Broker or fail the test if there is an error.
func (c *Client) CreateBrokerV1Beta1OrFail(name string, options ...resources.BrokerV1Beta1Option) *v1beta1.Broker {
	namespace := c.Namespace
	broker := resources.BrokerV1Beta1(name, options...)
	brokers := c.Eventing.EventingV1beta1().Brokers(namespace)
	c.T.Logf("Creating broker %s", name)
	// update broker with the new reference
	broker, err := brokers.Create(broker)
	if err != nil {
		c.T.Fatalf("Failed to create broker %q: %v", name, err)
	}
	c.Tracker.AddObj(broker)
	return broker
}

// CreateBrokersOrFail will create a list of Brokers.
func (c *Client) CreateBrokersOrFail(names []string, channelTypeMeta *metav1.TypeMeta) {
	c.T.Logf("Creating brokers %v", names)
	for _, name := range names {
		c.CreateBrokerOrFail(name, resources.WithChannelTemplateForBroker(channelTypeMeta))
	}
}

// CreateTriggerOrFail will create a Trigger or fail the test if there is an error.
func (c *Client) CreateTriggerOrFail(name string, options ...resources.TriggerOption) *v1alpha1.Trigger {
	namespace := c.Namespace
	trigger := resources.Trigger(name, options...)
	triggers := c.Eventing.EventingV1alpha1().Triggers(namespace)
	c.T.Logf("Creating trigger %s", name)
	// update trigger with the new reference
	trigger, err := triggers.Create(trigger)
	if err != nil {
		c.T.Fatalf("Failed to create trigger %q: %v", name, err)
	}
	c.Tracker.AddObj(trigger)
	return trigger
}

// CreateTriggerOrFailV1Beta1 will create a v1beta1 Trigger or fail the test if there is an error.
func (c *Client) CreateTriggerOrFailV1Beta1(name string, options ...resources.TriggerOptionV1Beta1) *v1beta1.Trigger {
	namespace := c.Namespace
	trigger := resources.TriggerV1Beta1(name, options...)
	triggers := c.Eventing.EventingV1beta1().Triggers(namespace)
	c.T.Logf("Creating v1beta1 trigger %s", name)
	// update trigger with the new reference
	trigger, err := triggers.Create(trigger)
	if err != nil {
		c.T.Fatalf("Failed to create v1beta1 trigger %q: %v", name, err)
	}
	c.Tracker.AddObj(trigger)
	return trigger
}

// CreateFlowsSequenceOrFail will create a Sequence (in flows.knative.dev api group) or
// fail the test if there is an error.
func (c *Client) CreateFlowsSequenceOrFail(sequence *flowsv1alpha1.Sequence) {
	c.T.Logf("Creating flows sequence %+v", sequence)
	sequences := c.Eventing.FlowsV1alpha1().Sequences(c.Namespace)
	_, err := sequences.Create(sequence)
	if err != nil {
		c.T.Fatalf("Failed to create flows sequence %q: %v", sequence.Name, err)
	}
	c.Tracker.AddObj(sequence)
}

// CreateFlowsParallelOrFail will create a Parallel (in flows.knative.dev api group) or
// fail the test if there is an error.
func (c *Client) CreateFlowsParallelOrFail(parallel *flowsv1alpha1.Parallel) {
	c.T.Logf("Creating flows parallel %+v", parallel)
	parallels := c.Eventing.FlowsV1alpha1().Parallels(c.Namespace)
	_, err := parallels.Create(parallel)
	if err != nil {
		c.T.Fatalf("Failed to create flows parallel %q: %v", parallel.Name, err)
	}
	c.Tracker.AddObj(parallel)
}

// CreateSinkBindingV1Alpha1OrFail will create a SinkBinding or fail the test if there is an error.
func (c *Client) CreateSinkBindingV1Alpha1OrFail(sb *sourcesv1alpha1.SinkBinding) {
	c.T.Logf("Creating sinkbinding %+v", sb)
	sbInterface := c.Eventing.SourcesV1alpha1().SinkBindings(c.Namespace)
	_, err := sbInterface.Create(sb)
	if err != nil {
		c.T.Fatalf("Failed to create sinkbinding %q: %v", sb.Name, err)
	}
	c.Tracker.AddObj(sb)
}

// CreateSinkBindingV1Alpha2OrFail will create a SinkBinding or fail the test if there is an error.
func (c *Client) CreateSinkBindingV1Alpha2OrFail(sb *sourcesv1alpha2.SinkBinding) {
	c.T.Logf("Creating sinkbinding %+v", sb)
	sbInterface := c.Eventing.SourcesV1alpha2().SinkBindings(c.Namespace)
	_, err := sbInterface.Create(sb)
	if err != nil {
		c.T.Fatalf("Failed to create sinkbinding %q: %v", sb.Name, err)
	}
	c.Tracker.AddObj(sb)
}

// CreateApiServerSourceOrFail will create an ApiServerSource
func (c *Client) CreateApiServerSourceOrFail(apiServerSource *sourcesv1alpha2.ApiServerSource) {
	c.T.Logf("Creating apiserversource %+v", apiServerSource)
	apiServerInterface := c.Eventing.SourcesV1alpha2().ApiServerSources(c.Namespace)
	_, err := apiServerInterface.Create(apiServerSource)
	if err != nil {
		c.T.Fatalf("Failed to create apiserversource %q: %v", apiServerSource.Name, err)
	}
	c.Tracker.AddObj(apiServerSource)
}

// CreateContainerSourceV1Alpha2OrFail will create a ContainerSource.
func (c *Client) CreateContainerSourceV1Alpha2OrFail(containerSource *sourcesv1alpha2.ContainerSource) {
	c.T.Logf("Creating containersource %+v", containerSource)
	containerInterface := c.Eventing.SourcesV1alpha2().ContainerSources(c.Namespace)
	_, err := containerInterface.Create(containerSource)
	if err != nil {
		c.T.Fatalf("Failed to create containersource %q: %v", containerSource.Name, err)
	}
	c.Tracker.AddObj(containerSource)
}

// CreatePingSourceV1Alpha1OrFail will create an PingSource
func (c *Client) CreatePingSourceV1Alpha1OrFail(pingSource *sourcesv1alpha1.PingSource) {
	c.T.Logf("Creating pingsource %+v", pingSource)
	pingInterface := c.Eventing.SourcesV1alpha1().PingSources(c.Namespace)
	_, err := pingInterface.Create(pingSource)
	if err != nil {
		c.T.Fatalf("Failed to create pingsource %q: %v", pingSource.Name, err)
	}
	c.Tracker.AddObj(pingSource)
}

// CreatePingSourceV1Alpha2OrFail will create an PingSource
func (c *Client) CreatePingSourceV1Alpha2OrFail(pingSource *sourcesv1alpha2.PingSource) {
	c.T.Logf("Creating pingsource %+v", pingSource)
	pingInterface := c.Eventing.SourcesV1alpha2().PingSources(c.Namespace)
	_, err := pingInterface.Create(pingSource)
	if err != nil {
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
	c.T.Logf("Creating pod %+v", pod)
	if _, err := c.Kube.CreatePod(pod); err != nil {
		c.T.Fatalf("Failed to create pod %q: %v", pod.Name, err)
	}
	c.Tracker.Add(coreAPIGroup, coreAPIVersion, "pods", namespace, pod.Name)
	c.podsCreated = append(c.podsCreated, pod.Name)
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
