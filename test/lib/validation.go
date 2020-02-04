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
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// The interval and timeout used for polling pod logs.
	interval = 1 * time.Second
	timeout  = 4 * time.Minute
)

// GetLog gets the logs from the given Pod in the namespace of this client. It will get the logs
// from the first container, whichever it is.
func (client *Client) GetLog(podName string) (string, error) {
	containerName, err := client.getContainerName(podName, client.Namespace)
	if err != nil {
		return "", err
	}
	logs, err := client.Kube.PodLogs(podName, containerName, client.Namespace)
	if err != nil {
		return "", err
	}
	return string(logs), nil
}

// CheckLog waits until logs for the logger Pod satisfy the checker.
// If the checker does not pass within timeout it returns error.
func (client *Client) CheckLog(podName string, checker func(string) bool) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		logs, err := client.GetLog(podName)
		if err != nil {
			return true, err
		}
		return checker(logs), nil
	})
}

// CheckLogEmpty waits the given amount of time and check the log is empty
func (client *Client) CheckLogEmpty(podName string, timeout time.Duration) error {
	time.Sleep(timeout)
	logs, err := client.GetLog(podName)
	if err != nil {
		return err
	}
	if logs != "" {
		return fmt.Errorf("expected empty log, got %s", logs)
	}
	return nil
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

// CheckerContainsCount returns a checker function to check if the log contains the count number of given content.
func CheckerContainsCount(content string, count int) func(string) bool {
	return func(log string) bool {
		return strings.Count(log, content) == count
	}
}

// CheckerContainsAtLeast returns a checker function to check if the log contains at least the count number of given content.
func CheckerContainsAtLeast(content string, count int) func(string) bool {
	return func(log string) bool {
		return strings.Count(log, content) >= count
	}
}

// FindAnyLogContents attempts to find logs for given Pod/Container that has 'any' of the given contents.
// It returns an error if it couldn't retrieve the logs. In case 'any' of the contents are there, it returns true.
func (client *Client) FindAnyLogContents(podName string, contents []string) (bool, error) {
	logs, err := client.GetLog(podName)
	if err != nil {
		return false, err
	}
	eventContentsSet, err := parseEventContentsFromPodLogs(logs)
	if err != nil {
		return false, err
	}
	for _, content := range contents {
		if eventContentsSet.Has(content) {
			return true, nil
		}
	}
	return false, nil
}

// parseEventContentsFromPodLogs extracts the contents of events from a Pod logs
// Example log entry: 2019/08/21 22:46:38 {"msg":"Body-type1-source1--extname1-extval1-extname2-extvalue2","sequence":"1"}
// Use regex to get the event content with json format: {"msg":"Body-type1-source1--extname1-extval1-extname2-extvalue2","sequence":"1"}
// Get the eventContent with key "msg"
// Returns a set with all unique event contents
func parseEventContentsFromPodLogs(logs string) (sets.String, error) {
	re := regexp.MustCompile(`{.+}`)
	matches := re.FindAllString(logs, -1)
	eventContentsSet := sets.String{}
	for _, match := range matches {
		var matchedLogs map[string]string
		err := json.Unmarshal([]byte(match), &matchedLogs)
		if err != nil {
			return nil, err
		} else {
			eventContent := matchedLogs["msg"]
			eventContentsSet.Insert(eventContent)
		}
	}
	return eventContentsSet, nil
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

// CheckConfigMapsExist will check if copy configmaps exist.
func (client *Client) CheckConfigMapsExist(namespace string, names ...string) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		for _, name := range names {
			_, err := client.Kube.Kube.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
			if k8serrors.IsNotFound(err) {
				return false, nil
			} else if err != nil {
				return false, err
			}
		}
		return true, nil
	})
}

// CheckConfigMapsEqual will check if configmaps have the same data as the original one.
func (client *Client) CheckConfigMapsEqual(originalNamespace, cmp string, names ...string) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		for _, name := range names {
			// Get original configmap
			origianlCM, err := client.Kube.Kube.CoreV1().ConfigMaps(originalNamespace).Get(name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			// Get copy configmap
			copyCM, err := client.Kube.Kube.CoreV1().ConfigMaps(client.Namespace).Get(cmp+"-"+name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if !reflect.DeepEqual(origianlCM.Data, copyCM.Data) {
				return false, nil
			}
		}
		return true, nil
	})
}
