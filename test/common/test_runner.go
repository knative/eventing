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
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
	pkgTest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"

	// Mysteriously required to support GCP auth (required by k8s libs).
	// Apparently just importing it is enough. @_@ side effects @_@.
	// https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

const TestPullSecretName = "kn-eventing-test-pull-secret"

// ChannelTestRunner is used to run tests against channels.
type ChannelTestRunner struct {
	ChannelFeatureMap map[metav1.TypeMeta][]Feature
	ChannelsToTest    []metav1.TypeMeta
}

// RunTests will use all channels that support the given feature, to run
// a test for the testFunc.
func (tr *ChannelTestRunner) RunTests(
	t *testing.T,
	feature Feature,
	testFunc func(st *testing.T, channel metav1.TypeMeta),
) {
	t.Parallel()
	for _, channel := range tr.ChannelsToTest {
		// If a Channel is not present in the map, then assume it has all properties. This is so an
		// unknown Channel can be specified via the --channel flag and have tests run.
		// TODO Use a flag to specify the features of the flag based Channel, rather than assuming
		// it supports all features.
		features, present := tr.ChannelFeatureMap[channel]
		if !present || contains(features, feature) {
			t.Run(fmt.Sprintf("%s-%s", t.Name(), channel), func(st *testing.T) {
				testFunc(st, channel)
			})
		}
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

// Setup creates the client objects needed in the e2e tests,
// and does other setups, like creating namespaces, set the test case to run in parallel, etc.
func Setup(t *testing.T, runInParallel bool) *Client {
	// Create a new namespace to run this test case.
	baseFuncName := helpers.GetBaseFuncName(t.Name())
	namespace := makeK8sNamespace(baseFuncName)
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
	//                Investigate if there is other way to gracefully terminate the tests in the middle.
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

func makeK8sNamespace(baseFuncName string) string {
	base := helpers.MakeK8sNamePrefix(baseFuncName)
	return names.SimpleNameGenerator.GenerateName(base + "-")
}

// TearDown will delete created names using clients.
func TearDown(client *Client) {
	client.Tracker.Clean(true)
	if err := DeleteNameSpace(client); err != nil {
		client.T.Logf("Could not delete the namespace %q: %v", client.Namespace, err)
	}
}

// CopySecret will copy a secret from one namespace into another.
// If a ServiceAccount name is provided then it'll add it as a PullSecret to
// it.
// It'll either return a pointer to the new Secret or and error indicating
// why it couldn't do it.
func CopySecret(client *Client, srcNS string, srcSecretName string, tgtNS string, svcAccount string) (*corev1.Secret, error) {
	// Get the Interfaces we need to access the resources in the cluster
	srcSecI := client.Kube.Kube.CoreV1().Secrets(srcNS)
	tgtNSSvcAccI := client.Kube.Kube.CoreV1().ServiceAccounts(tgtNS)
	tgtNSSecI := client.Kube.Kube.CoreV1().Secrets(tgtNS)

	// First try to find the secret we're supposed to copy
	srcSecret, err := srcSecI.Get(srcSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Just double checking
	if srcSecret == nil {
		return nil, errors.New("error copying Secret, it's nil w/o error")
	}

	// Found the secret, so now make a copy in our new namespace
	newSecret, err := tgtNSSecI.Create(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: srcSecretName,
			},
			Data: srcSecret.Data,
			Type: srcSecret.Type,
		})

	// If the secret already exists then that's ok - some other test
	// must have created it
	if err != nil && !apierrs.IsAlreadyExists(err) {
		return nil, fmt.Errorf("error copying the Secret: %s", err)
	}

	client.T.Logf("Copied Secret %q into Namespace %q",
		srcSecretName, tgtNS)

	// If a ServiceAccount was provided then add it as an ImagePullSecret.
	// Note: if the SA aleady has it then this is no-op
	if svcAccount != "" {
		_, err = tgtNSSvcAccI.Patch(svcAccount, types.StrategicMergePatchType,
			[]byte(`{"imagePullSecrets":[{"name":"`+srcSecretName+`"}]}`))
		if err != nil {
			return nil, fmt.Errorf("patch failed on NS/SA (%s/%s): %s",
				tgtNS, srcSecretName, err)
		}
		client.T.Logf("Added Secret %q as ImagePullSecret to SA %q in NS %q",
			srcSecretName, svcAccount, tgtNS)
	}

	return newSecret, nil
}

// CreateNamespaceIfNeeded creates a new namespace if it does not exist.
func CreateNamespaceIfNeeded(t *testing.T, client *Client, namespace string) {
	_, err := client.Kube.Kube.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})

	if err != nil && apierrs.IsNotFound(err) {
		nsSpec := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		_, err = client.Kube.Kube.CoreV1().Namespaces().Create(nsSpec)

		if err != nil {
			t.Fatalf("Failed to create Namespace: %s; %v", namespace, err)
		}

		// https://github.com/kubernetes/kubernetes/issues/66689
		// We can only start creating pods after the default ServiceAccount is created by the kube-controller-manager.
		err = waitForServiceAccountExists(t, client, "default", namespace)
		if err != nil {
			t.Fatalf("The default ServiceAccount was not created for the Namespace: %s", namespace)
		}

		// If the "default" Namespace has a secret called
		// "kn-eventing-test-pull-secret" then use that as the ImagePullSecret
		// on the "default" ServiceAccount in this new Namespace.
		// This is needed for cases where the images are in a private registry.
		_, err := CopySecret(client, "default", TestPullSecretName, namespace, "default")
		if err != nil && !apierrs.IsNotFound(err) {
			t.Fatalf("error copying the secret into ns %q: %s", namespace, err)
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
	if err == nil || !apierrs.IsNotFound(err) {
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
