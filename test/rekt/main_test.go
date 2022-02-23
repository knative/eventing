//go:build e2e
// +build e2e

/*
Copyright 2020 The Knative Authors

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

package rekt

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/test/helpers"
	"knative.dev/pkg/test/prow"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"knative.dev/pkg/system"

	"knative.dev/pkg/injection"
	_ "knative.dev/pkg/system/testing"

	"knative.dev/reconciler-test/pkg/environment"
)

// global is the singleton instance of GlobalEnvironment. It is used to parse
// the testing config for the test run. The config will specify the cluster
// config as well as the parsing level and state flags.
var global environment.GlobalEnvironment

func init() {
	// environment.InitFlags registers state and level filter flags.
	environment.InitFlags(flag.CommandLine)
}

// TestMain is the first entry point for `go test`.
func TestMain(m *testing.M) {
	// We get a chance to parse flags to include the framework flags for the
	// framework as well as any additional flags included in the integration.
	flag.Parse()

	// EnableInjectionOrDie will enable client injection, this is used by the
	// testing framework for namespace management, and could be leveraged by
	// features to pull Kubernetes clients or the test environment out of the
	// context passed in the features.
	ctx, startInformers := injection.EnableInjectionOrDie(nil, nil) //nolint
	startInformers()

	// global is used to make instances of Environments, NewGlobalEnvironment
	// is passing and saving the client injection enabled context for use later.
	global = environment.NewGlobalEnvironment(ctx)

	// Run the tests.
	exit := m.Run()

	// Collect logs only when test failed.
	if exit != 0 {
		ExportLogs(ctx, "knative-eventing", system.Namespace())
	}

	os.Exit(exit)
}

func exportLogs(kubeClient kubernetes.Interface, namespace, dir string, logFunc func(format string, args ...interface{})) error {

	// Create a directory for the namespace.
	logPath := filepath.Join(dir, namespace)
	if err := helpers.CreateDir(logPath); err != nil {
		return fmt.Errorf("error creating directory %q: %w", namespace, err)
	}

	pods, err := kubeClient.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing pods in namespace %q: %w", namespace, err)
	}

	var errs []error
	for _, pod := range pods.Items {
		for _, ct := range pod.Spec.Containers {
			fn := filepath.Join(logPath, fmt.Sprintf("%s-%s.log", pod.Name, ct.Name))
			logFunc("Exporting logs in pod %q container %q to %q", pod.Name, ct.Name, fn)
			f, err := os.Create(fn)
			if err != nil {
				errs = append(errs, fmt.Errorf("error creating file %q: %w", fn, err))
			}
			log, err := podLogs(context.Background(), kubeClient, pod.Name, ct.Name, pod.Namespace)
			if err != nil {
				errs = append(errs, fmt.Errorf("error getting logs for pod %q container %q: %w", pod.Name, ct.Name, err))
			}
			_, err = f.Write(log)
			if err != nil {
				errs = append(errs, fmt.Errorf("error writing logs into file %q: %w", fn, err))
			}

			f.Close()
		}
	}

	return helpers.CombineErrors(errs)
}

func ExportLogs(ctx context.Context, systemLogsDir, systemNamespace string) {
	// If the test is run by CI, export the pod logs in the namespace to the artifacts directory,
	// which will then be uploaded to GCS after the test job finishes.
	if prow.IsCI() {
		kubeClient := kubeclient.Get(ctx)
		dir := filepath.Join(prow.GetLocalArtifactsDir(), systemLogsDir)
		if err := exportLogs(kubeClient, systemNamespace, dir, log.Printf); err != nil {
			log.Printf("Error in exporting logs: %v", err)
		}
	}
}

// PodLogs returns Pod logs for given Pod and Container in the namespace
func podLogs(ctx context.Context, client kubernetes.Interface, podName, containerName, namespace string) ([]byte, error) {
	pods := client.CoreV1().Pods(namespace)
	podList, err := pods.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range podList.Items {
		// Pods are big, so avoid copying.
		pod := &podList.Items[i]
		if strings.Contains(pod.Name, podName) {
			result := pods.GetLogs(pod.Name, &corev1.PodLogOptions{
				Container: containerName,
			}).Do(ctx)
			return result.Raw()
		}
	}
	return nil, fmt.Errorf("could not find logs for %s/%s:%s", namespace, podName, containerName)
}
