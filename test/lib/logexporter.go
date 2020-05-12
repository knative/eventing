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
	"fmt"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/test/helpers"
)

func (c *Client) ExportLogs(dir string) error {
	// Create a directory for the namespace.
	logPath := filepath.Join(dir, c.Namespace)
	if err := helpers.CreateDir(logPath); err != nil {
		return fmt.Errorf("error creating directory %q: %w", c.Namespace, err)
	}

	pods, err := c.Kube.Kube.CoreV1().Pods(c.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing pods in namespace %q: %w", c.Namespace, err)
	}

	var errs []error
	for _, pod := range pods.Items {
		for _, ct := range pod.Spec.Containers {
			fn := filepath.Join(logPath, fmt.Sprintf("%s-%s.log", pod.Name, ct.Name))
			c.T.Logf("Exporting logs in pod %q container %q to %q", pod.Name, ct.Name, fn)
			f, err := os.Create(fn)
			if err != nil {
				errs = append(errs, fmt.Errorf("error creating file %q: %w", fn, err))
			}
			log, err := c.Kube.PodLogs(pod.Name, ct.Name, c.Namespace)
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
