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

package k8s

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func GetFirstTerminationMessage(pod *corev1.Pod) string {
	if pod != nil {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Terminated != nil && cs.State.Terminated.Message != "" {
				return cs.State.Terminated.Message
			}
		}
	}
	return ""
}

func GetOperationsResult(ctx context.Context, pod *corev1.Pod, result interface{}) error {
	if pod == nil {
		return fmt.Errorf("pod was nil")
	}
	terminationMessage := GetFirstTerminationMessage(pod)
	if terminationMessage == "" {
		return fmt.Errorf("did not find termination message for pod %q", pod.Name)
	}
	err := json.Unmarshal([]byte(terminationMessage), &result)
	if err != nil {
		return fmt.Errorf("failed to unmarshal terminationmessage: %q : %q", terminationMessage, err)
	}
	return nil
}
