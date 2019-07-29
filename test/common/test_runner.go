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
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	pkgTest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"

	// Mysteriously required to support GCP auth (required by k8s libs).
	// Apparently just importing it is enough. @_@ side effects @_@.
	// https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// RunTests will use all channels that support the given feature, to run
// a test for the testFunc.
func RunTests(
	t *testing.T,
	channels []string,
	feature Feature,
	testFunc func(st *testing.T, channel string),
) {
	t.Parallel()
	for _, channel := range channels {
		channelConfig := ValidChannelsMap[channel]
		if contains(channelConfig.Features, feature) {
			t.Run(fmt.Sprintf("%s-%s", t.Name(), channel), func(st *testing.T) {
				testFunc(st, channel)
			})
		}
	}
}

// Setup creates the client objects needed in the e2e tests,
// and does other setups, like creating namespaces, set the test case to run in parallel, etc.
func Setup(t *testing.T, runInParallel bool) *Client {
	// Create a new namespace to run this test case.
	baseFuncName := helpers.GetBaseFuncName(t.Name())
	namespace := helpers.MakeK8sNamePrefix(baseFuncName)
	t.Logf("namespace is : %q", namespace)
	client, err := NewClient(
		pkgTest.Flags.Kubeconfig,
		pkgTest.Flags.Cluster,
		namespace,
		t)
	if err != nil {
		t.Fatalf("Couldn't initialize clients: %v", err)
	}

	CreateNamespaceIfNeeded(t, client, namespace)

	// Disallow manually interrupting the tests.
	// TODO(Fredy-Z): t.Skip() can only be called on its own goroutine.
	//                Investigate if there is other way to gracefully terminte the tests in the middle.
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Printf("Test %q running, please don't interrupt...\n", t.Name())
	}()

	// Run the test case in parallel if needed.
	if runInParallel {
		t.Parallel()
	}

	return client
}

// TearDown will delete created names using clients.
func TearDown(client *Client) {
	client.Tracker.Clean(true)
	if err := DeleteNameSpace(client); err != nil {
		client.T.Logf("Could not delete the namespace %q: %v", client.Namespace, err)
	}
}

func contains(features []Feature, feature Feature) bool {
	for _, f := range features {
		if f == feature {
			return true
		}
	}
	return false
}

// GetChannelTypeMeta gets the actual typemeta of the channel.
func GetChannelTypeMeta(channelName string) *metav1.TypeMeta {
	return MessagingTypeMeta(channelName)
}

// CreateNamespaceIfNeeded creates a new namespace if it does not exist.
func CreateNamespaceIfNeeded(t *testing.T, client *Client, namespace string) {
	nsSpec, err := client.Kube.Kube.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})

	if err != nil && errors.IsNotFound(err) {
		nsSpec = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		nsSpec, err = client.Kube.Kube.CoreV1().Namespaces().Create(nsSpec)

		if err != nil {
			t.Fatalf("Failed to create Namespace: %s; %v", namespace, err)
		}

		// https://github.com/kubernetes/kubernetes/issues/66689
		// We can only start creating pods after the default ServiceAccount is created by the kube-controller-manager.
		err = waitForServiceAccountExists(t, client, "default", namespace)
		if err != nil {
			t.Fatalf("The default ServiceAccount was not created for the Namespace: %s", namespace)
		}
	}
}

// waitForServiceAccountExists waits until the ServiceAccount exists.
func waitForServiceAccountExists(t *testing.T, client *Client, name, namespace string) error {
	return wait.PollImmediate(1*time.Second, 2*time.Minute, func() (bool, error) {
		sas := client.Kube.Kube.CoreV1().ServiceAccounts(namespace)
		if _, err := sas.Get(name, metav1.GetOptions{}); err == nil {
			return true, nil
		}
		return false, nil
	})
}

// DeleteNameSpace deletes the namespace that has the given name.
func DeleteNameSpace(client *Client) error {
	_, err := client.Kube.Kube.CoreV1().Namespaces().Get(client.Namespace, metav1.GetOptions{})
	if err == nil || !errors.IsNotFound(err) {
		return client.Kube.Kube.CoreV1().Namespaces().Delete(client.Namespace, nil)
	}
	return err
}

// LogPodLogsForDebugging add the pod logs in the testing log for further debugging.
func LogPodLogsForDebugging(client *Client, podName, containerName string) {
	namespace := client.Namespace
	logs, err := client.Kube.PodLogs(podName, containerName, namespace)
	if err != nil {
		client.T.Logf("Failed to get the logs for container %q of the pod %q in namespace %q: %v", containerName, podName, namespace, err)
	} else {
		client.T.Logf("Logs for the container %q of the pod %q in namespace %q:\n%s", containerName, podName, namespace, string(logs))
	}
}
