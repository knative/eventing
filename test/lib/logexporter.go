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

package lib

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	pkgtest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
	"knative.dev/pkg/test/logstream/v2"
	"knative.dev/pkg/test/prow"
)

func (c *Client) ExportLogs(dir string) error {
	return exportLogs(c.Kube, c.Namespace, dir, c.T.Logf)
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
			log, err := pkgtest.PodLogs(context.Background(), kubeClient, pod.Name, ct.Name, pod.Namespace)
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

func ExportLogs(systemLogsDir, systemNamespace string) {

	// If the test is run by CI, export the pod logs in the namespace to the artifacts directory,
	// which will then be uploaded to GCS after the test job finishes.
	if prow.IsCI() {
		config, err := pkgtest.Flags.GetRESTConfig()
		if err != nil {
			log.Printf("Failed to create REST config: %v\n", err)
			return
		}

		kubeClient, err := kubernetes.NewForConfig(config)
		if err != nil {
			log.Printf("Failed to create kube client: %v\n", err)
			return
		}

		dir := filepath.Join(prow.GetLocalArtifactsDir(), systemLogsDir)
		if err := exportLogs(kubeClient, systemNamespace, dir, log.Printf); err != nil {
			log.Printf("Error in exporting logs: %v", err)
		}
	}
}

// ExportLogStreamOnError starts a log stream from the given namespace
// for pods specified via podPrefixes. The log strema is stopped and
// logs exported upon calling the returned Canceler.
func ExportLogStreamOnError(t *testing.T, logDir, namespace string, podPrefixes ...string) logstream.Canceler {
	config, err := pkgtest.Flags.GetRESTConfig()
	if err != nil {
		t.Fatalf("Failed to create REST config: %v\n", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("Failed to create kube client: %v\n", err)
	}

	buf := threadSafeBuffer{}
	callback := func(s string, params ...interface{}) {
		buf.Write([]byte(fmt.Sprintf(s, params...) + "\n"))
	}

	sysStream := logstream.New(context.Background(), kubeClient,
		logstream.WithNamespaces(namespace),
		logstream.WithLineFiltering(false),
		logstream.WithPodPrefixes(podPrefixes...))
	canceler, err := sysStream.StartStream("unfiltered-logs", callback)
	if err != nil {
		t.Fatal("Unable to stream logs from namespace", err)
	}

	return func() {
		canceler()

		if !t.Failed() {
			return
		}

		// Maps pod name prefix to its logs.
		logs := make(map[string]string)
		if len(podPrefixes) == 0 {
			logs["all-in-one"] = buf.String()
		} else {
			// Divide logs by podPrefix so that logs for each pod are stored in a separate file.
			for _, podPrefix := range podPrefixes {
				for _, line := range strings.Split(buf.String(), "\n") {
					if strings.Contains(line, podPrefix) {
						logs[podPrefix] = logs[podPrefix] + line + "\n"
					}
				}
			}
		}

		dir := filepath.Join(prow.GetLocalArtifactsDir(), logDir)
		if err := helpers.CreateDir(dir); err != nil {
			t.Errorf("Error creating directory %q: %v", dir, err)
		}

		var errs []error
		for podPrefix, log := range logs {
			path := filepath.Join(dir, fmt.Sprintf("%s.log", podPrefix))
			f, err := os.Create(path)
			if err != nil {
				errs = append(errs, fmt.Errorf("error creating file %q: %w", path, err))
				continue
			}
			_, err = f.Write([]byte(log))
			if err != nil {
				errs = append(errs, fmt.Errorf("error writing logs into file %q: %w", path, err))
			}
			_ = f.Close()
		}

		if len(errs) > 0 {
			t.Errorf("Failed to write logs: %v", helpers.CombineErrors(errs))
		}
	}
}

// threadSafeBuffer avoids race conditions on bytes.Buffer.
// See: https://stackoverflow.com/a/36226525/844449
type threadSafeBuffer struct {
	bytes.Buffer
	sync.Mutex
}

func (b *threadSafeBuffer) Write(p []byte) (n int, err error) {
	b.Mutex.Lock()
	defer b.Mutex.Unlock()
	return b.Buffer.Write(p)
}
