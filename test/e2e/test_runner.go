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
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode"

	"github.com/knative/eventing/test"
	"github.com/knative/eventing/test/common"
	pkgTest "github.com/knative/pkg/test"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	// Mysteriously required to support GCP auth (required by k8s libs).
	// Apparently just importing it is enough. @_@ side effects @_@.
	// https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// RunTests will use all provisioners that support the given feature, to run
// a test for the testFunc.
func RunTests(t *testing.T, feature common.Feature, testFunc func(st *testing.T, provisioner string, isCRD bool)) {
	t.Parallel()
	for _, provisioner := range test.EventingFlags.Provisioners {
		channelConfig := common.ValidProvisionersMap[provisioner]
		if contains(channelConfig.Features, feature) {
			t.Run(fmt.Sprintf("%s-%s", t.Name(), provisioner), func(st *testing.T) {
				testFunc(st, provisioner, false)
			})

			if channelConfig.CRDSupported {
				t.Run(fmt.Sprintf("%s-crd-%s", t.Name(), provisioner), func(st *testing.T) {
					testFunc(st, provisioner, true)
				})
			}
		}
	}
}

// Setup creates the client objects needed in the e2e tests,
// and does other setups, like creating namespaces, set the test case to run in parallel, etc.
func Setup(t *testing.T, runInParallel bool) *common.Client {
	// Create a new namespace to run this test case.
	baseFuncName := getBaseFuncName(t.Name())
	namespace := makeK8sNamePrefix(baseFuncName)
	t.Logf("namespace is : %q", namespace)
	client, err := common.NewClient(
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
func TearDown(client *common.Client) {
	client.Cleaner.Clean(true)
	if err := DeleteNameSpace(client); err != nil {
		client.T.Logf("Could not delete the namespace %q: %v", client.Namespace, err)
	}
}

func contains(features []common.Feature, feature common.Feature) bool {
	for _, f := range features {
		if f == feature {
			return true
		}
	}
	return false
}

// Get the actual typemeta of the Channel type.
// TODO(Fredy-Z): This function is a workaround when there are both provisioner and Channel CRD in this repo.
//                It needs to be removed when the provisioner implementation is removed.
func getChannelTypeMeta(provisioner string, isCRD bool) *metav1.TypeMeta {
	channelTypeMeta := common.ChannelTypeMeta
	if isCRD {
		channelTypeMeta = common.OperatorChannelMap[provisioner]
	}
	return channelTypeMeta
}

// CreateNamespaceIfNeeded creates a new namespace if it does not exist.
func CreateNamespaceIfNeeded(t *testing.T, client *common.Client, namespace string) {
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
func waitForServiceAccountExists(t *testing.T, client *common.Client, name, namespace string) error {
	return wait.PollImmediate(1*time.Second, 2*time.Minute, func() (bool, error) {
		sas := client.Kube.Kube.CoreV1().ServiceAccounts(namespace)
		if _, err := sas.Get(name, metav1.GetOptions{}); err == nil {
			return true, nil
		}
		return false, nil
	})
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

// getBaseFuncName returns the baseFuncName parsed from the fullFuncName.
// eg. test/e2e.TestMain will return TestMain.
// TODO(Fredy-Z): many functions in this file can be moved to knative/pkg/test to make it cleaner.
func getBaseFuncName(fullFuncName string) string {
	baseFuncName := fullFuncName[strings.LastIndex(fullFuncName, "/")+1:]
	baseFuncName = baseFuncName[strings.LastIndex(baseFuncName, ".")+1:]
	return baseFuncName
}

// DeleteNameSpace deletes the namespace that has the given name.
func DeleteNameSpace(client *common.Client) error {
	_, err := client.Kube.Kube.CoreV1().Namespaces().Get(client.Namespace, metav1.GetOptions{})
	if err == nil || !errors.IsNotFound(err) {
		return client.Kube.Kube.CoreV1().Namespaces().Delete(client.Namespace, nil)
	}
	return err
}

// logPodLogsForDebugging add the pod logs in the testing log for further debugging.
func logPodLogsForDebugging(client *common.Client, podName, containerName string) {
	namespace := client.Namespace
	logs, err := client.Kube.PodLogs(podName, containerName, namespace)
	if err != nil {
		client.T.Logf("Failed to get the logs for container %q of the pod %q in namespace %q: %v", containerName, podName, namespace, err)
	} else {
		client.T.Logf("Logs for the container %q of the pod %q in namespace %q:\n%s", containerName, podName, namespace, string(logs))
	}
}
