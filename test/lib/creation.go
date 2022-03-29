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
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/reconciler"
	pkgtest "knative.dev/pkg/test"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	flowsv1 "knative.dev/eventing/pkg/apis/flows/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	sourcesv1beta2 "knative.dev/eventing/pkg/apis/sources/v1beta2"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
	ti "knative.dev/eventing/test/test_images"
)

// TODO(chizhg): break this file into multiple files when it grows too large.

var coreAPIGroup = corev1.SchemeGroupVersion.Group
var coreAPIVersion = corev1.SchemeGroupVersion.Version
var rbacAPIGroup = rbacv1.SchemeGroupVersion.Group
var rbacAPIVersion = rbacv1.SchemeGroupVersion.Version

func (c *Client) RetryWebhookErrors(updater func(int) error) error {
	return duck.RetryWebhookErrors(updater)
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

// CreateChannelV1WithDefaultOrFail will create a default Channel Resource in Eventing or fail the test if there is an error.
func (c *Client) CreateChannelWithDefaultOrFail(channel *messagingv1.Channel) {
	c.T.Logf("Creating default v1 channel %+v", channel)
	channels := c.Eventing.MessagingV1().Channels(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := channels.Create(context.Background(), channel, metav1.CreateOptions{})
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

// CreateSubscriptionV1OrFail will create a v1 Subscription or fail the test if there is an error.
func (c *Client) CreateSubscriptionOrFail(
	name, channelName string,
	channelTypeMeta *metav1.TypeMeta,
	options ...resources.SubscriptionOption,
) *messagingv1.Subscription {
	namespace := c.Namespace
	subscription := resources.Subscription(name, channelName, channelTypeMeta, options...)
	var retSubscription *messagingv1.Subscription
	subscriptions := c.Eventing.MessagingV1().Subscriptions(namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		c.T.Logf("Creating v1 subscription %s for channel %+v-%s", name, channelTypeMeta, channelName)
		// update subscription with the new reference
		var e error
		retSubscription, e = subscriptions.Create(context.Background(), subscription, metav1.CreateOptions{})
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
			retSubscription, e = subscriptions.Get(context.Background(), name, metav1.GetOptions{})
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

// CreateSubscriptionsV1OrFail will create a list of v1 Subscriptions with the same configuration except the name.
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

// CreateConfigMapOrFail will create a configmap or fail the test if there is an error.
func (c *Client) CreateConfigMapOrFail(name, namespace string, data map[string]string) *corev1.ConfigMap {
	c.T.Logf("Creating configmap %s", name)
	configMap, err := c.Kube.CoreV1().ConfigMaps(namespace).Create(context.Background(), resources.ConfigMap(name, namespace, data), metav1.CreateOptions{})
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
			"channel-template-spec": fmt.Sprintf(`
      apiVersion: %q
      kind: %q
`, channel.APIVersion, channel.Kind),
		},
	}
	_, err := c.Kube.CoreV1().ConfigMaps(c.Namespace).Create(context.Background(), cm, metav1.CreateOptions{})
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

// CreateBrokerV1OrFail will create a v1 Broker or fail the test if there is an error.
func (c *Client) CreateBrokerOrFail(name string, options ...resources.BrokerOption) *eventingv1.Broker {
	namespace := c.Namespace
	broker := resources.Broker(name, options...)
	var retBroker *eventingv1.Broker
	brokers := c.Eventing.EventingV1().Brokers(namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		c.T.Logf("Creating v1 broker %s", name)
		// update broker with the new reference
		var e error
		retBroker, e = brokers.Create(context.Background(), broker, metav1.CreateOptions{})
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
			retBroker, e = brokers.Get(context.Background(), name, metav1.GetOptions{})
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

// CreateTriggerOrFail will create a v1 Trigger or fail the test if there is an error.
func (c *Client) CreateTriggerOrFail(name string, options ...resources.TriggerOption) *eventingv1.Trigger {
	namespace := c.Namespace
	trigger := resources.Trigger(name, options...)
	var retTrigger *eventingv1.Trigger
	triggers := c.Eventing.EventingV1().Triggers(namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		c.T.Logf("Creating v1 trigger %s", name)
		// update trigger with the new reference
		var e error
		retTrigger, e = triggers.Create(context.Background(), trigger, metav1.CreateOptions{})
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
			retTrigger, e = triggers.Get(context.Background(), name, metav1.GetOptions{})
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
func (c *Client) CreateFlowsSequenceOrFail(sequence *flowsv1.Sequence) {
	c.T.Logf("Creating flows sequence %+v", sequence)
	sequences := c.Eventing.FlowsV1().Sequences(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := sequences.Create(context.Background(), sequence, metav1.CreateOptions{})
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
func (c *Client) CreateFlowsParallelOrFail(parallel *flowsv1.Parallel) {
	c.T.Logf("Creating flows parallel %+v", parallel)
	parallels := c.Eventing.FlowsV1().Parallels(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := parallels.Create(context.Background(), parallel, metav1.CreateOptions{})
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

// CreateSinkBindingV1OrFail will create a SinkBinding or fail the test if there is an error.
func (c *Client) CreateSinkBindingV1OrFail(sb *sourcesv1.SinkBinding) {
	c.T.Logf("Creating sinkbinding %+v", sb)
	sbInterface := c.Eventing.SourcesV1().SinkBindings(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := sbInterface.Create(context.Background(), sb, metav1.CreateOptions{})
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

// CreateApiServerSourceV1OrFail will create an v1 ApiServerSource
func (c *Client) CreateApiServerSourceV1OrFail(apiServerSource *sourcesv1.ApiServerSource) {
	c.T.Logf("Creating apiserversource %+v", apiServerSource)
	apiServerInterface := c.Eventing.SourcesV1().ApiServerSources(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := apiServerInterface.Create(context.Background(), apiServerSource, metav1.CreateOptions{})
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

// CreateContainerSourceV1OrFail will create a v1 ContainerSource.
func (c *Client) CreateContainerSourceV1OrFail(containerSource *sourcesv1.ContainerSource) {
	c.T.Logf("Creating containersource %+v", containerSource)
	containerInterface := c.Eventing.SourcesV1().ContainerSources(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := containerInterface.Create(context.Background(), containerSource, metav1.CreateOptions{})
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

// CreatePingSourceV1Beta2OrFail will create a PingSource
func (c *Client) CreatePingSourceV1Beta2OrFail(pingSource *sourcesv1beta2.PingSource) {
	c.T.Logf("Creating pingsource %+v", pingSource)
	pingInterface := c.Eventing.SourcesV1beta2().PingSources(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := pingInterface.Create(context.Background(), pingSource, metav1.CreateOptions{})
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

// CreatePingSourceV1OrFail will create a PingSource
func (c *Client) CreatePingSourceV1OrFail(pingSource *sourcesv1.PingSource) {
	c.T.Logf("Creating pingsource %+v", pingSource)
	pingInterface := c.Eventing.SourcesV1().PingSources(c.Namespace)
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		_, e := pingInterface.Create(context.Background(), pingSource, metav1.CreateOptions{})
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
	if newSvc, err := c.Kube.CoreV1().Services(namespace).Create(context.Background(), svc, metav1.CreateOptions{}); err != nil {
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

		svcs := client.Kube.CoreV1().Services(namespace)
		if _, err := svcs.Create(context.Background(), svc, metav1.CreateOptions{}); err != nil {
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

	c.applyAdditionalEnv(&pod.Spec)

	// the following retryable errors are expected when creating a blank Pod:
	// - update conflicts
	// - "No API token found for service account %q,
	//    retry after the token is automatically created and added to the service account"
	err := reconciler.RetryErrors(func(attempts int) (err error) {
		c.T.Logf("Creating pod %+v", pod)
		_, e := pkgtest.CreatePod(context.Background(), c.Kube, pod)
		return e
	}, apierrs.IsConflict, apierrs.IsServerTimeout)
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

	c.applyAdditionalEnv(&deploy.Spec.Template.Spec)

	c.T.Logf("Creating deployment %+v", deploy)
	if _, err := c.Kube.AppsV1().Deployments(deploy.Namespace).Create(context.Background(), deploy, metav1.CreateOptions{}); err != nil {
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

	c.applyAdditionalEnv(&cronjob.Spec.JobTemplate.Spec.Template.Spec)

	c.T.Logf("Creating cronjob %+v", cronjob)
	if _, err := c.Kube.BatchV1beta1().CronJobs(cronjob.Namespace).Create(context.Background(), cronjob, metav1.CreateOptions{}); err != nil {
		c.T.Fatalf("Failed to create cronjob %q: %v", cronjob.Name, err)
	}
	c.Tracker.Add("batch", "v1beta1", "cronjobs", namespace, cronjob.Name)
}

// CreateServiceAccountOrFail will create a ServiceAccount or fail the test if there is an error.
func (c *Client) CreateServiceAccountOrFail(saName string) {
	namespace := c.Namespace
	sa := resources.ServiceAccount(saName, namespace)
	sas := c.Kube.CoreV1().ServiceAccounts(namespace)
	c.T.Logf("Creating service account %+v", sa)
	if _, err := sas.Create(context.Background(), sa, metav1.CreateOptions{}); err != nil && !apierrs.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create service account %q: %v", saName, err)
	}
	c.Tracker.Add(coreAPIGroup, coreAPIVersion, "serviceaccounts", namespace, saName)

	// If the "default" Namespace has a secret called
	// "kn-eventing-test-pull-secret" then use that as the ImagePullSecret
	// on the new ServiceAccount we just created.
	// This is needed for cases where the images are in a private registry.
	_, err := utils.CopySecret(c.Kube.CoreV1(), "default", testPullSecretName, namespace, saName)
	if err != nil && !errors.IsNotFound(err) {
		c.T.Fatalf("Error copying the secret: %s", err)
	}
}

// CreateClusterRoleOrFail creates the given ClusterRole or fail the test if there is an error.
func (c *Client) CreateClusterRoleOrFail(cr *rbacv1.ClusterRole) {
	c.T.Logf("Creating cluster role %+v", cr)
	crs := c.Kube.RbacV1().ClusterRoles()
	if _, err := crs.Create(context.Background(), cr, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create cluster role %q: %v", cr.Name, err)
	}
	c.Tracker.Add(rbacAPIGroup, rbacAPIVersion, "clusterroles", "", cr.Name)
}

// CreateRoleOrFail creates the given Role in the Client namespace or fail the test if there is an error.
func (c *Client) CreateRoleOrFail(r *rbacv1.Role) {
	c.T.Logf("Creating role %+v", r)
	namespace := c.Namespace
	rs := c.Kube.RbacV1().Roles(namespace)
	if _, err := rs.Create(context.Background(), r, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create role %q: %v", r.Name, err)
	}
	c.Tracker.Add(rbacAPIGroup, rbacAPIVersion, "roles", namespace, r.Name)
}

const (
	RoleKind = "Role"
)

// CreateRoleBindingOrFail will create a RoleBinding or fail the test if there is an error.
func (c *Client) CreateRoleBindingOrFail(saName, rKind, rName, rbName, rbNamespace string) {
	saNamespace := c.Namespace
	rb := resources.RoleBinding(saName, saNamespace, rKind, rName, rbName, rbNamespace)
	rbs := c.Kube.RbacV1().RoleBindings(rbNamespace)

	c.T.Logf("Creating role binding %+v", rb)
	if _, err := rbs.Create(context.Background(), rb, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create role binding %q: %v", rbName, err)
	}
	c.Tracker.Add(rbacAPIGroup, rbacAPIVersion, "rolebindings", rbNamespace, rb.GetName())
}

// CreateClusterRoleBindingOrFail will create a ClusterRoleBinding or fail the test if there is an error.
func (c *Client) CreateClusterRoleBindingOrFail(saName, crName, crbName string) {
	saNamespace := c.Namespace
	crb := resources.ClusterRoleBinding(saName, saNamespace, crName, crbName)
	crbs := c.Kube.RbacV1().ClusterRoleBindings()

	c.T.Logf("Creating cluster role binding %+v", crb)
	if _, err := crbs.Create(context.Background(), crb, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		c.T.Fatalf("Failed to create cluster role binding %q: %v", crbName, err)
	}
	c.Tracker.Add(rbacAPIGroup, rbacAPIVersion, "clusterrolebindings", "", crb.GetName())
}

func (c *Client) applyAdditionalEnv(pod *corev1.PodSpec) {
	for i := 0; i < len(pod.Containers); i++ {
		pod.Containers[i].Env = append(pod.Containers[i].Env, corev1.EnvVar{Name: ti.ConfigTracingEnv, Value: c.TracingCfg})
		if c.loggingCfg != "" {
			pod.Containers[i].Env = append(pod.Containers[i].Env, corev1.EnvVar{Name: ti.ConfigLoggingEnv, Value: c.loggingCfg})
		}
	}
}

func CreateRBACPodsEventsGetListWatch(client *Client, name string) {
	client.CreateServiceAccountOrFail(name)
	client.CreateRoleOrFail(resources.Role(name,
		resources.WithRuleForRole(&rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"pods", "events"},
			Verbs:     []string{"get", "list", "watch"}}),
	))
	client.CreateRoleBindingOrFail(name, RoleKind, name, name, client.Namespace)
}

func CreateRBACPodsGetEventsAll(client *Client, name string) {
	client.CreateServiceAccountOrFail(name)
	client.CreateRoleOrFail(resources.Role(name,
		resources.WithRuleForRole(&rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get"},
		}),
		resources.WithRuleForRole(&rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{rbacv1.VerbAll},
		}),
	))
	client.CreateRoleBindingOrFail(name, RoleKind, name, name, client.Namespace)
}
