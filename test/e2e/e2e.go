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
	"sync"
	"testing"
	"time"
	"unicode"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/test"
	pkgTest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	// Mysteriously required to support GCP auth (required by k8s libs).
	// Apparently just importing it is enough. @_@ side effects @_@.
	// https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

type clusterChannelProvisioner struct {
	sync.Mutex
	name string
}

func (ccp *clusterChannelProvisioner) Get() string {
	ccp.Lock()
	name := ccp.name
	ccp.Unlock()
	return name
}

func (ccp *clusterChannelProvisioner) Set(name string) {
	ccp.Lock()
	ccp.name = name
	ccp.Unlock()
}

// ClusterChannelProvisionerToTest hold the CCP that is used to run the test case.
// It's default to be the first one that is passed through the clusterChannelProvisioners flag.
// And it can be changed in main_test.go for test case setup.
var ClusterChannelProvisionerToTest = clusterChannelProvisioner{name: test.EventingFlags.Provisioners[0]}

const (
	interval = 1 * time.Second
	timeout  = 2 * time.Minute
)

// Setup validates namespace and provisioner, creates the client objects needed in the e2e tests.
func Setup(t *testing.T, runInParallel bool, logf logging.FormatLogger) (*test.Clients, string, string, *test.Cleaner) {
	clients, err := test.NewClients(
		pkgTest.Flags.Kubeconfig,
		pkgTest.Flags.Cluster)
	if err != nil {
		t.Fatalf("Couldn't initialize clients: %v", err)
	}
	cleaner := test.NewCleaner(logf, clients.Dynamic)

	// Get the CCP to run this test case.
	ccpToTest := ClusterChannelProvisionerToTest.Get()

	// Create a new namespace to run this test case.
	// Combine the test name and CCP to avoid duplication.
	baseFuncName := GetBaseFuncName(t.Name())
	ns := makeK8sNamePrefix(baseFuncName)
	CreateNamespaceIfNeeded(t, clients, ns, t.Logf)

	// Run the test case in parallel if needed.
	if runInParallel {
		t.Parallel()
	}

	return clients, ns, ccpToTest, cleaner
}

// TODO(Fredy-Z): Borrowed this function from Knative/Serving, will delete it after we move it to Knative/pkg/test.
// makeK8sNamePrefix converts each chunk of non-alphanumeric character into a single dash
// and also convert camelcase tokens into dash-delimited lowercase tokens.
func makeK8sNamePrefix(s string) string {
	var sb strings.Builder
	newToken := false
	for _, c := range s {
		if !(unicode.IsLetter(c) || unicode.IsNumber(c)) {
			newToken = true
			continue
		}
		if sb.Len() > 0 && (newToken || unicode.IsUpper(c)) {
			sb.WriteRune('-')
		}
		sb.WriteRune(unicode.ToLower(c))
		newToken = false
	}
	return sb.String()
}

// GetBaseFuncName returns the baseFuncName parsed from the fullFuncName.
// eg. test/e2e.TestMain will return TestMain.
// TODO(Fredy-Z): many functions in this file can be moved to knative/pkg/test to make it cleaner.
func GetBaseFuncName(fullFuncName string) string {
	baseFuncName := fullFuncName[strings.LastIndex(fullFuncName, "/")+1:]
	baseFuncName = baseFuncName[strings.LastIndex(baseFuncName, ".")+1:]
	return baseFuncName
}

// TearDown will delete created names using clients.
func TearDown(clients *test.Clients, namespace string, cleaner *test.Cleaner, logf logging.FormatLogger) {
	cleaner.Clean(true)
	if err := DeleteNameSpace(clients, namespace); err != nil {
		logf("Could not delete the namespace %q: %v", namespace, err)
	}
}

// CreateChannel will create a Channel.
func CreateChannel(clients *test.Clients, channel *v1alpha1.Channel, _ logging.FormatLogger, cleaner *test.Cleaner) error {
	namespace := channel.Namespace
	channels := clients.Eventing.EventingV1alpha1().Channels(namespace)
	res, err := channels.Create(channel)
	if err != nil {
		return err
	}
	cleaner.Add(v1alpha1.SchemeGroupVersion.Group, v1alpha1.SchemeGroupVersion.Version, "channels", namespace, res.ObjectMeta.Name)
	return nil
}

// CreateSubscription will create a Subscription.
func CreateSubscription(clients *test.Clients, sub *v1alpha1.Subscription, _ logging.FormatLogger, cleaner *test.Cleaner) error {
	namespace := sub.Namespace
	subscriptions := clients.Eventing.EventingV1alpha1().Subscriptions(namespace)
	res, err := subscriptions.Create(sub)
	if err != nil {
		return err
	}
	cleaner.Add(v1alpha1.SchemeGroupVersion.Group, v1alpha1.SchemeGroupVersion.Version, "subscriptions", namespace, res.ObjectMeta.Name)
	return nil
}

// WithChannelsAndSubscriptionsReady creates Channels and Subscriptions and waits until all are Ready.
// When they are ready, chans and subs are altered to get the real Channels and Subscriptions.
func WithChannelsAndSubscriptionsReady(clients *test.Clients, namespace string, chans *[]*v1alpha1.Channel, subs *[]*v1alpha1.Subscription, logf logging.FormatLogger, cleaner *test.Cleaner) error {
	for _, channel := range *chans {
		if err := CreateChannel(clients, channel, logf, cleaner); err != nil {
			return err
		}
	}

	channels := clients.Eventing.EventingV1alpha1().Channels(namespace)
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

	subscriptions := clients.Eventing.EventingV1alpha1().Subscriptions(namespace)
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
	namespace := broker.Namespace
	brokers := clients.Eventing.EventingV1alpha1().Brokers(namespace)
	res, err := brokers.Create(broker)
	if err != nil {
		return err
	}
	cleaner.Add(v1alpha1.SchemeGroupVersion.Group, v1alpha1.SchemeGroupVersion.Version, "brokers", namespace, res.ObjectMeta.Name)
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
	namespace := trigger.Namespace
	triggers := clients.Eventing.EventingV1alpha1().Triggers(namespace)
	res, err := triggers.Create(trigger)
	if err != nil {
		return err
	}
	cleaner.Add(v1alpha1.SchemeGroupVersion.Group, v1alpha1.SchemeGroupVersion.Version, "triggers", namespace, res.ObjectMeta.Name)
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

// CreateServiceAccount will create a service account.
func CreateServiceAccount(clients *test.Clients, sa *corev1.ServiceAccount, _ logging.FormatLogger, cleaner *test.Cleaner) error {
	namespace := sa.Namespace
	sas := clients.Kube.Kube.CoreV1().ServiceAccounts(namespace)
	res, err := sas.Create(sa)
	if err != nil {
		return err
	}
	cleaner.Add(corev1.SchemeGroupVersion.Group, corev1.SchemeGroupVersion.Version, "serviceaccounts", namespace, res.ObjectMeta.Name)
	return nil
}

// CreateClusterRoleBinding will create a service account binding.
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
// cluster-admin role.
func CreateServiceAccountAndBinding(clients *test.Clients, saName, crName, namespace string, logf logging.FormatLogger, cleaner *test.Cleaner) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: namespace,
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
			Name:     crName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	err = CreateClusterRoleBinding(clients, crb, logf, cleaner)
	if err != nil {
		return err
	}
	return nil
}

// CreatePodAndServiceReady will create a Pod and Service, and wait for them to become ready.
func CreatePodAndServiceReady(clients *test.Clients, pod *corev1.Pod, svc *corev1.Service, logf logging.FormatLogger, cleaner *test.Cleaner) (*corev1.Pod, error) {
	namespace := pod.Namespace
	if err := CreatePod(clients, pod, logf, cleaner); err != nil {
		return nil, fmt.Errorf("Failed to create pod: %v", err)
	}
	if err := pkgTest.WaitForPodRunning(clients.Kube, pod.Name, namespace); err != nil {
		return nil, fmt.Errorf("Error waiting for pod to become running: %v", err)
	}
	logf("Pod %q starts running", pod.Name)

	if err := CreateService(clients, svc, logf, cleaner); err != nil {
		return nil, fmt.Errorf("Failed to create service: %v", err)
	}

	// Reload pod to get IP.
	pod, err := clients.Kube.Kube.CoreV1().Pods(namespace).Get(pod.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Failed to get pod: %v", err)
	}

	// FIXME(Fredy-Z): This hacky sleep is added to try mitigating the test flakiness. Will delete it after we find the root cause and fix.
	time.Sleep(10 * time.Second)

	return pod, nil
}

// CreateService will create a Service.
func CreateService(clients *test.Clients, svc *corev1.Service, _ logging.FormatLogger, cleaner *test.Cleaner) error {
	namespace := svc.Namespace
	svcs := clients.Kube.Kube.CoreV1().Services(namespace)
	res, err := svcs.Create(svc)
	if err != nil {
		return err
	}
	cleaner.Add(corev1.SchemeGroupVersion.Group, corev1.SchemeGroupVersion.Version, "services", namespace, res.ObjectMeta.Name)
	return nil
}

// CreatePod will create a Pod.
func CreatePod(clients *test.Clients, pod *corev1.Pod, _ logging.FormatLogger, cleaner *test.Cleaner) error {
	res, err := clients.Kube.CreatePod(pod)
	if err != nil {
		return err
	}
	cleaner.Add(corev1.SchemeGroupVersion.Group, corev1.SchemeGroupVersion.Version, "pods", res.ObjectMeta.Namespace, res.ObjectMeta.Name)
	return nil
}

// SendFakeEventToChannel will create fake CloudEvent and send it to the given channel.
func SendFakeEventToChannel(clients *test.Clients, event *test.CloudEvent, channel *v1alpha1.Channel, logf logging.FormatLogger, cleaner *test.Cleaner) error {
	namespace := channel.Namespace
	url := fmt.Sprintf("http://%s", channel.Status.Address.Hostname)
	return sendFakeEventToAddress(clients, event, url, namespace, logf, cleaner)
}

// SendFakeEventToBroker will create fake CloudEvent and send it to the given broker.
func SendFakeEventToBroker(clients *test.Clients, event *test.CloudEvent, broker *v1alpha1.Broker, logf logging.FormatLogger, cleaner *test.Cleaner) error {
	namespace := broker.Namespace
	url := fmt.Sprintf("http://%s", broker.Status.Address.Hostname)
	return sendFakeEventToAddress(clients, event, url, namespace, logf, cleaner)
}

func sendFakeEventToAddress(clients *test.Clients, event *test.CloudEvent, url, namespace string, logf logging.FormatLogger, cleaner *test.Cleaner) error {
	logf("Sending fake CloudEvent")
	logf("Creating event sender pod %q", event.Source)

	pod := test.EventSenderPod(event.Source, namespace, url, event)
	if err := CreatePod(clients, pod, logf, cleaner); err != nil {
		return err
	}
	if err := pkgTest.WaitForPodRunning(clients.Kube, pod.Name, namespace); err != nil {
		return err
	}
	logf("Sender pod starts running")
	return nil
}

// WaitForLogContents waits until logs for given Pod/Container include the given contents.
// If the contents are not present within timeout it returns error.
func WaitForLogContents(clients *test.Clients, logf logging.FormatLogger, podName string, containerName string, namespace string, contents []string) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		logs, err := clients.Kube.PodLogs(podName, containerName, namespace)
		if err != nil {
			return true, err
		}
		for _, content := range contents {
			if !strings.Contains(string(logs), content) {
				logf("Could not find content %q for %s/%s. Found %q instead", content, podName, containerName, string(logs))
				return false, nil
			}
			// Do not return as we will keep on looking for the other contents in the slice.
			logf("Found content %q for %s/%s in logs %q", content, podName, containerName, string(logs))
		}
		return true, nil
	})
}

// WaitForLogContentCount checks if the number of substr occur times equals the given number.
// If the content does not appear the given times it returns error.
func WaitForLogContentCount(client *test.Clients, podName, containerName, namespace, content string, appearTimes int) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		logs, err := client.Kube.PodLogs(podName, containerName, namespace)
		if err != nil {
			return true, err
		}

		return strings.Count(string(logs), content) == appearTimes, nil
	})
}

// FindAnyLogContents attempts to find logs for given Pod/Container that has 'any' of the given contents.
// It returns an error if it couldn't retrieve the logs. In case 'any' of the contents are there, it returns true.
func FindAnyLogContents(clients *test.Clients, logf logging.FormatLogger, podName string, containerName string, namespace string, contents []string) (bool, error) {
	logs, err := clients.Kube.PodLogs(podName, containerName, namespace)
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
func WaitForAllTriggersReady(clients *test.Clients, namespace string, logf logging.FormatLogger) error {
	triggers := clients.Eventing.EventingV1alpha1().Triggers(namespace)
	if err := test.WaitForTriggersListState(triggers, test.TriggersReady, "TriggerIsReady"); err != nil {
		return err
	}
	return nil
}

// CreateNamespaceIfNeeded creates a new namespace if it does not exist.
func CreateNamespaceIfNeeded(t *testing.T, clients *test.Clients, namespace string, logf logging.FormatLogger) {
	nsSpec, err := clients.Kube.Kube.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})

	if err != nil && errors.IsNotFound(err) {
		nsSpec = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		logf("Creating Namespace: %s", namespace)
		nsSpec, err = clients.Kube.Kube.CoreV1().Namespaces().Create(nsSpec)

		if err != nil {
			t.Fatalf("Failed to create Namespace: %s; %v", namespace, err)
		}

		// https://github.com/kubernetes/kubernetes/issues/66689
		// We can only start creating pods after the default ServiceAccount is created by the kube-controller-manager.
		err = WaitForServiceAccountExists(t, clients, "default", namespace, logf)
		if err != nil {
			t.Fatalf("The default ServiceAccount was not created for the Namespace: %s", namespace)
		}
	}
}

// WaitForServiceAccountExists waits until the ServiceAccount exists.
func WaitForServiceAccountExists(t *testing.T, clients *test.Clients, name, namespace string, logf logging.FormatLogger) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		sas := clients.Kube.Kube.CoreV1().ServiceAccounts(namespace)
		if _, err := sas.Get(name, metav1.GetOptions{}); err == nil {
			return true, nil
		}
		return false, nil
	})
}

// LabelNamespace labels the given namespace with the labels map.
func LabelNamespace(clients *test.Clients, namespace string, labels map[string]string, logf logging.FormatLogger) error {
	nsSpec, err := clients.Kube.Kube.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
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

// DeleteNameSpace deletes the namespace that has the given name.
func DeleteNameSpace(clients *test.Clients, namespace string) error {
	_, err := clients.Kube.Kube.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	if err == nil || !errors.IsNotFound(err) {
		return clients.Kube.Kube.CoreV1().Namespaces().Delete(namespace, nil)
	}
	return err
}

// logPodLogsForDebugging add the pod logs in the testing log for further debugging.
func logPodLogsForDebugging(clients *test.Clients, podName, containerName, namespace string, logf logging.FormatLogger) {
	logs, err := clients.Kube.PodLogs(podName, containerName, namespace)
	if err != nil {
		logf("Failed to get the logs for container %q of the pod %q in namespace %q: %v", containerName, podName, namespace, err)
	} else {
		logf("Logs for the container %q of the pod %q in namespace %q:\n%s", containerName, podName, namespace, string(logs))
	}
}
