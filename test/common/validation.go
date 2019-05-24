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
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// The interval and timeout used for polling pod logs.
	interval = 1 * time.Second
	timeout  = 2 * time.Minute
)

// CheckLog waits until logs for the logger Pod satisfy the checker.
// If the checker does not pass within timeout it returns error.
func (client *Client) CheckLog(podName string, checker func(string) bool) error {
	namespace := client.Namespace
	containerName, err := client.getContainerName(podName, namespace)
	if err != nil {
		return err
	}
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		logs, err := client.Kube.PodLogs(podName, containerName, namespace)
		if err != nil {
			return true, err
		}
		return checker(string(logs)), nil
	})
}

// CheckerContains returns a checker function to check if the log contains the given content.
func CheckerContains(content string) func(string) bool {
	return func(log string) bool {
		return strings.Contains(log, content)
	}
}

// CheckerContainsAll returns a checker function to check if the log contains all the given contents.
func CheckerContainsAll(contents []string) func(string) bool {
	return func(log string) bool {
		for _, content := range contents {
			if !strings.Contains(log, content) {
				return false
			}
		}
		return true
	}
}

// CheckerContainsCount returns a checker functions to check if the log contains the count number of given content.
func CheckerContainsCount(content string, count int) func(string) bool {
	return func(log string) bool {
		return strings.Count(log, content) == count
	}
}

// FindAnyLogContents attempts to find logs for given Pod/Container that has 'any' of the given contents.
// It returns an error if it couldn't retrieve the logs. In case 'any' of the contents are there, it returns true.
func (client *Client) FindAnyLogContents(podName string, contents []string) (bool, error) {
	namespace := client.Namespace
	containerName, err := client.getContainerName(podName, namespace)
	if err != nil {
		return false, err
	}
	logs, err := client.Kube.PodLogs(podName, containerName, namespace)
	if err != nil {
		return false, err
	}
	for _, content := range contents {
		if strings.Contains(string(logs), content) {
			return true, nil
		}
	}
	return false, nil
}

// getContainerName gets name of the first container of the given pod.
// Now our logger pod only contains one single container, and is only used for receiving events and validation.
func (client *Client) getContainerName(podName, namespace string) (string, error) {
	pod, err := client.Kube.Kube.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	containerName := pod.Spec.Containers[0].Name
	return containerName, nil
}
