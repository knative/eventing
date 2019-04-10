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

package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/test"
	pkgTest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	// Mysteriously required to support GCP auth (required by k8s libs).
	// Apparently just importing it is enough. @_@ side effects @_@.
	// https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const (
	DefaultTestNamespace = "e2etest-knative-eventing"

	interval = 1 * time.Second
	timeout  = 1 * time.Minute
)

// Setup creates the client objects needed in the e2e tests.
func Setup(t *testing.T, logf logging.FormatLogger) (*test.Clients, *test.Cleaner) {
	if pkgTest.Flags.Namespace == "" {
		pkgTest.Flags.Namespace = DefaultTestNamespace
	}

	clients, err := test.NewClients(
		pkgTest.Flags.Kubeconfig,
		pkgTest.Flags.Cluster,
		pkgTest.Flags.Namespace)
	if err != nil {
		t.Fatalf("Couldn't initialize clients: %v", err)
	}
	cleaner := test.NewCleaner(logf, clients.Dynamic)

	return clients, cleaner
}

// TearDown will delete created names using clients.
func TearDown(clients *test.Clients, cleaner *test.Cleaner, _ logging.FormatLogger) {
	cleaner.Clean(true)
}

// CreateChannel will create a Channel
func CreateChannel(clients *test.Clients, channel *v1alpha1.Channel, _ logging.FormatLogger, cleaner *test.Cleaner) error {
	channels := clients.Eventing.EventingV1alpha1().Channels(pkgTest.Flags.Namespace)
	res, err := channels.Create(channel)
	if err != nil {
		return err
	}
	cleaner.Add(v1alpha1.SchemeGroupVersion.Group, v1alpha1.SchemeGroupVersion.Version, "channels", pkgTest.Flags.Namespace, res.ObjectMeta.Name)
	return nil
}

// CreateSubscription will create a Subscription
func CreateSubscription(clients *test.Clients, sub *v1alpha1.Subscription, _ logging.FormatLogger, cleaner *test.Cleaner) error {
	subscriptions := clients.Eventing.EventingV1alpha1().Subscriptions(pkgTest.Flags.Namespace)
	res, err := subscriptions.Create(sub)
	if err != nil {
		return err
	}
	cleaner.Add(v1alpha1.SchemeGroupVersion.Group, v1alpha1.SchemeGroupVersion.Version, "subscriptions", pkgTest.Flags.Namespace, res.ObjectMeta.Name)
	return nil
}

// WithChannelsAndSubscriptionsReady creates Channels and Subscriptions and waits until all are Ready.
// When they are ready, chans and subs are altered to get the real Channels and Subscriptions.
func WithChannelsAndSubscriptionsReady(clients *test.Clients, chans *[]*v1alpha1.Channel, subs *[]*v1alpha1.Subscription, logf logging.FormatLogger, cleaner *test.Cleaner) error {
	for _, channel := range *chans {
		if err := CreateChannel(clients, channel, logf, cleaner); err != nil {
			return err
		}
	}

	channels := clients.Eventing.EventingV1alpha1().Channels(pkgTest.Flags.Namespace)
	for i, channel := range *chans {
		if err := test.WaitForChannelState(channels, channel.Name, test.IsChannelReady, "ChannelIsReady"); err != nil {
			return err
		}
		// Update the given object so they'll reflect the ready state.
		updatedchannel, err := channels.Get(channel.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		updatedchannel.DeepCopyInto(channel)
		(*chans)[i] = channel
	}

	for _, sub := range *subs {
		if err := CreateSubscription(clients, sub, logf, cleaner); err != nil {
			return err
		}
	}

	subscriptions := clients.Eventing.EventingV1alpha1().Subscriptions(pkgTest.Flags.Namespace)
	for i, sub := range *subs {
		if err := test.WaitForSubscriptionState(subscriptions, sub.Name, test.IsSubscriptionReady, "SubscriptionIsReady"); err != nil {
			return err
		}
		// Update the given object so they'll reflect the ready state.
		updatedsub, err := subscriptions.Get(sub.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		updatedsub.DeepCopyInto(sub)
		(*subs)[i] = sub
	}

	return nil
}

// CreateBroker will create a Broker.
func CreateBroker(clients *test.Clients, broker *v1alpha1.Broker, logf logging.FormatLogger, cleaner *test.Cleaner) error {
	brokers := clients.Eventing.EventingV1alpha1().Brokers(broker.Namespace)
	res, err := brokers.Create(broker)
	if err != nil {
		return err
	}
	cleaner.Add(v1alpha1.SchemeGroupVersion.Group, v1alpha1.SchemeGroupVersion.Version, "brokers", broker.Namespace, res.ObjectMeta.Name)
	return nil
}

// WithBrokerReady creates a Broker and waits until it is Ready.
func WithBrokerReady(clients *test.Clients, broker *v1alpha1.Broker, logf logging.FormatLogger, cleaner *test.Cleaner) error {
	if err := CreateBroker(clients, broker, logf, cleaner); err != nil {
		return err
	}
	return WaitForBrokerReady(clients, broker)
}

// WaitForBrokerReady waits until the broker is Ready.
func WaitForBrokerReady(clients *test.Clients, broker *v1alpha1.Broker) error {
	brokers := clients.Eventing.EventingV1alpha1().Brokers(broker.Namespace)
	if err := test.WaitForBrokerState(brokers, broker.Name, test.IsBrokerReady, "BrokerIsReady"); err != nil {
		return err
	}
	// Update the given object so they'll reflect the ready state.
	updatedBroker, err := brokers.Get(broker.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	updatedBroker.DeepCopyInto(broker)
	return nil
}

// CreateTrigger will create a Trigger.
func CreateTrigger(clients *test.Clients, trigger *v1alpha1.Trigger, logf logging.FormatLogger, cleaner *test.Cleaner) error {
	triggers := clients.Eventing.EventingV1alpha1().Triggers(trigger.Namespace)
	res, err := triggers.Create(trigger)
	if err != nil {
		return err
	}
	cleaner.Add(v1alpha1.SchemeGroupVersion.Group, v1alpha1.SchemeGroupVersion.Version, "triggers", trigger.Namespace, res.ObjectMeta.Name)
	return nil
}

// WithTriggerReady creates a Trigger and waits until it is Ready.
func WithTriggerReady(clients *test.Clients, trigger *v1alpha1.Trigger, logf logging.FormatLogger, cleaner *test.Cleaner) error {
	if err := CreateTrigger(clients, trigger, logf, cleaner); err != nil {
		return err
	}

	triggers := clients.Eventing.EventingV1alpha1().Triggers(trigger.Namespace)
	if err := test.WaitForTriggerState(triggers, trigger.Name, test.IsTriggerReady, "TriggerIsReady"); err != nil {
		return err
	}
	// Update the given object so they'll reflect the ready state.
	updatedTrigger, err := triggers.Get(trigger.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	updatedTrigger.DeepCopyInto(trigger)

	return nil
}

// CreateServiceAccount will create a service account
func CreateServiceAccount(clients *test.Clients, sa *corev1.ServiceAccount, _ logging.FormatLogger, cleaner *test.Cleaner) error {
	sas := clients.Kube.Kube.CoreV1().ServiceAccounts(pkgTest.Flags.Namespace)
	res, err := sas.Create(sa)
	if err != nil {
		return err
	}
	cleaner.Add(corev1.SchemeGroupVersion.Group, corev1.SchemeGroupVersion.Version, "serviceaccounts", pkgTest.Flags.Namespace, res.ObjectMeta.Name)
	return nil
}

// CreateClusterRoleBinding will create a service account binding
func CreateClusterRoleBinding(clients *test.Clients, crb *rbacv1.ClusterRoleBinding, _ logging.FormatLogger, cleaner *test.Cleaner) error {
	clusterRoleBindings := clients.Kube.Kube.RbacV1().ClusterRoleBindings()
	res, err := clusterRoleBindings.Create(crb)
	if err != nil {
		return err
	}
	cleaner.Add(rbacv1.SchemeGroupVersion.Group, rbacv1.SchemeGroupVersion.Version, "clusterrolebindings", "", res.ObjectMeta.Name)
	return nil
}

// CreateServiceAccountAndBinding creates both ServiceAccount and ClusterRoleBinding with default
// cluster-admin role
func CreateServiceAccountAndBinding(clients *test.Clients, name string, logf logging.FormatLogger, cleaner *test.Cleaner) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: pkgTest.Flags.Namespace,
		},
	}
	err := CreateServiceAccount(clients, sa, logf, cleaner)
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
			Name:     "cluster-admin",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	err = CreateClusterRoleBinding(clients, crb, logf, cleaner)
	if err != nil {
		return err
	}
	return nil
}

// CreatePodAndServiceReady will create a Pod and Service, and wait for them to become ready
func CreatePodAndServiceReady(clients *test.Clients, pod *corev1.Pod, svc *corev1.Service, ns string, logf logging.FormatLogger, cleaner *test.Cleaner) (*corev1.Pod, error) {
	if err := CreatePod(clients, pod, logf, cleaner); err != nil {
		return nil, fmt.Errorf("Failed to create pod: %v", err)
	}
	if err := pkgTest.WaitForAllPodsRunning(clients.Kube, ns); err != nil {
		return nil, fmt.Errorf("Error waiting for pod to become running: %v", err)
	}
	logf("Pod %q starts running", pod.Name)

	if err := CreateService(clients, svc, logf, cleaner); err != nil {
		return nil, fmt.Errorf("Failed to create service: %v", err)
	}

	// Reload pod to get IP
	pod, err := clients.Kube.Kube.CoreV1().Pods(pod.Namespace).Get(pod.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Failed to get pod: %v", err)
	}

	return pod, nil
}

// CreateService will create a Service
func CreateService(clients *test.Clients, svc *corev1.Service, _ logging.FormatLogger, cleaner *test.Cleaner) error {
	svcs := clients.Kube.Kube.CoreV1().Services(svc.GetNamespace())
	res, err := svcs.Create(svc)
	if err != nil {
		return err
	}
	cleaner.Add(corev1.SchemeGroupVersion.Group, corev1.SchemeGroupVersion.Version, "services", res.ObjectMeta.Namespace, res.ObjectMeta.Name)
	return nil
}

// CreatePod will create a Pod
func CreatePod(clients *test.Clients, pod *corev1.Pod, _ logging.FormatLogger, cleaner *test.Cleaner) error {
	res, err := clients.Kube.CreatePod(pod)
	if err != nil {
		return err
	}
	cleaner.Add(corev1.SchemeGroupVersion.Group, corev1.SchemeGroupVersion.Version, "pods", res.ObjectMeta.Namespace, res.ObjectMeta.Name)
	return nil
}

// SendFakeEventToChannel will create fake CloudEvent and send it to the given channel.
func SendFakeEventToChannel(clients *test.Clients, event *test.CloudEvent, channel *v1alpha1.Channel, ns string, logf logging.FormatLogger, cleaner *test.Cleaner) error {
	logf("Sending fake CloudEvent")
	logf("Creating event sender pod")
	url := fmt.Sprintf("http://%s", channel.Status.Address.Hostname)
	pod := test.EventSenderPod(event.Source, ns, url, event)
	logf("Sender pod: %#v", pod)
	if err := CreatePod(clients, pod, logf, cleaner); err != nil {
		return err
	}
	if err := pkgTest.WaitForAllPodsRunning(clients.Kube, ns); err != nil {
		return err
	}
	logf("Sender pod starts running")
	return nil
}

// WaitForLogContents waits until logs for given Pod/Container include the given contents.
// If the contents are not present within timeout it returns error.
func WaitForLogContents(clients *test.Clients, logf logging.FormatLogger, podName string, containerName string, namespace string, contents []string) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		logs, err := clients.Kube.PodLogs(podName, containerName)
		if err != nil {
			return true, err
		}
		for _, content := range contents {
			if !strings.Contains(string(logs), content) {
				logf("Could not find content %q for %s/%s. Found %q instead", content, podName, containerName, string(logs))
				return false, nil
			}
			// do not return as we will keep on looking for the other contents in the slice
			logf("Found content %q for %s/%s in logs %q", content, podName, containerName, string(logs))
		}
		return true, nil
	})
}

// WaitForLogContentCount checks if the number of substr occur times equals the given number.
// If the content does not appear the given times it returns error.
func WaitForLogContentCount(client *test.Clients, podName, containerName, content string, appearTimes int) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		logs, err := client.Kube.PodLogs(podName, containerName)
		if err != nil {
			return true, err
		}

		return strings.Count(string(logs), content) == appearTimes, nil
	})
}

// FindAnyLogContents attempts to find logs for given Pod/Container that has 'any' of the given contents.
// It returns an error if it couldn't retrieve the logs. In case 'any' of the contents are there, it returns true.
func FindAnyLogContents(clients *test.Clients, logf logging.FormatLogger, podName string, containerName string, namespace string, contents []string) (bool, error) {
	logs, err := clients.Kube.PodLogs(podName, containerName)
	if err != nil {
		return false, err
	}
	for _, content := range contents {
		if strings.Contains(string(logs), content) {
			logf("Found content %q for %s/%s.", content, podName, containerName)
			return true, nil
		}
	}
	return false, nil
}

// WaitForAllTriggersReady will wait until all triggers in the given namespace are ready.
func WaitForAllTriggersReady(clients *test.Clients, logf logging.FormatLogger, namespace string) error {
	triggers := clients.Eventing.EventingV1alpha1().Triggers(namespace)
	if err := test.WaitForTriggersListState(triggers, test.TriggersReady, "TriggerIsReady"); err != nil {
		return err
	}
	return nil
}

// LabelNamespace labels the test namespace with the labels map.
func LabelNamespace(clients *test.Clients, logf logging.FormatLogger, labels map[string]string) error {
	ns := pkgTest.Flags.Namespace
	nsSpec, err := clients.Kube.Kube.CoreV1().Namespaces().Get(ns, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		return err
	}
	if nsSpec.Labels == nil {
		nsSpec.Labels = map[string]string{}
	}
	for k, v := range labels {
		nsSpec.Labels[k] = v
	}
	_, err = clients.Kube.Kube.CoreV1().Namespaces().Update(nsSpec)
	return err
}

// CreateNamespaceIfNeeded creates a new namespace if it does not exist
func CreateNamespaceIfNeeded(t *testing.T, clients *test.Clients, logf logging.FormatLogger) (string, func()) {
	shutdown := func() {}
	ns := pkgTest.Flags.Namespace
	logf("Namespace: %s", ns)

	nsSpec, err := clients.Kube.Kube.CoreV1().Namespaces().Get(ns, metav1.GetOptions{})

	if err != nil && errors.IsNotFound(err) {
		nsSpec = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
		logf("Creating Namespace: %s", ns)
		nsSpec, err = clients.Kube.Kube.CoreV1().Namespaces().Create(nsSpec)

		if err != nil {
			t.Fatalf("Failed to create Namespace: %s; %v", ns, err)
		} else {
			shutdown = func() {
				clients.Kube.Kube.CoreV1().Namespaces().Delete(nsSpec.Name, nil)
				// TODO: this is a bit hacky but in order for the tests to work
				// correctly for a clean namespace to be created we need to also
				// wait for it to be removed.
				// To fix this we could generate namespace names.
				// This only happens when the namespace provided does not exist.
				//
				// wait up to 120 seconds for the namespace to be removed.
				logf("Deleting Namespace: %s", ns)
				for i := 0; i < 120; i++ {
					time.Sleep(1 * time.Second)
					if _, err := clients.Kube.Kube.CoreV1().Namespaces().Get(ns, metav1.GetOptions{}); err != nil && errors.IsNotFound(err) {
						logf("Namespace has been deleted")
						// the namespace is gone.
						break
					}
				}
			}
		}
	}
	return ns, shutdown
}
