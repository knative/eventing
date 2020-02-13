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
	"testing"
	"time"

	"knative.dev/eventing/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// SetupClientOption does further setup for the Client. It can be used if other projects
// need to do extra setups to run the tests we expose as test helpers.
type SetupClientOption func(*Client)

// SetupClientOptionNoop is a SetupClientOption that does nothing.
var SetupClientOptionNoop SetupClientOption = func(*Client) {
	// nothing
}

// Setup creates the client objects needed in the e2e tests,
// and does other setups, like creating namespaces, set the test case to run in parallel, etc.
func Setup(t *testing.T, runInParallel bool, options ...SetupClientOption) *Client {
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

	// Run the test case in parallel if needed.
	if runInParallel {
		t.Parallel()
	}

	// Clean up resources if the test is interrupted in the middle.
	pkgTest.CleanupOnInterrupt(func() { TearDown(client) }, t.Logf)

	// Run further setups for the client.
	for _, option := range options {
		option(client)
	}

	return client
}

func makeK8sNamespace(baseFuncName string) string {
	base := helpers.MakeK8sNamePrefix(baseFuncName)
	return names.SimpleNameGenerator.GenerateName(base + "-")
}

// TearDown will delete created names using clients.
func TearDown(client *Client) {
	// Dump the events in the namespace
	el, err := client.Kube.Kube.CoreV1().Events(client.Namespace).List(metav1.ListOptions{})
	if err != nil {
		client.T.Logf("Could not list events in the namespace %q: %v", client.Namespace, err)
	} else {
		for _, e := range el.Items {
			client.T.Logf("EVENT: %v", e)
		}
	}
	client.Tracker.Clean(true)
	if err := DeleteNameSpace(client); err != nil {
		client.T.Logf("Could not delete the namespace %q: %v", client.Namespace, err)
	}
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
		err = waitForServiceAccountExists(client, "default", namespace)
		if err != nil {
			t.Fatalf("The default ServiceAccount was not created for the Namespace: %s", namespace)
		}

		// If the "default" Namespace has a secret called
		// "kn-eventing-test-pull-secret" then use that as the ImagePullSecret
		// on the "default" ServiceAccount in this new Namespace.
		// This is needed for cases where the images are in a private registry.
		_, err := utils.CopySecret(client.Kube.Kube.CoreV1(), "default", TestPullSecretName, namespace, "default")
		if err != nil && !apierrs.IsNotFound(err) {
			t.Fatalf("error copying the secret into ns %q: %s", namespace, err)
		}
	}
}

// waitForServiceAccountExists waits until the ServiceAccount exists.
func waitForServiceAccountExists(client *Client, name, namespace string) error {
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
