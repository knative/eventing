/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package performance

import (
	"fmt"
	"strings"
	"time"

	"github.com/knative/pkg/test"
	"github.com/knative/test-infra/shared/junit"
	"k8s.io/apimachinery/pkg/util/wait"

	// Mysteriously required to support GCP auth (required by k8s libs). Apparently just importing it is enough. @_@ side effects @_@. https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const (
	// Property name used by testgrid.
	perfLatency = "perf_latency"
)

// constants used in the pod logs, which we then use to build test results.
// TODO(Fredy-Z): we'll need a more robust way to export test results from the pod.
const (
	TestResultKey  = "RESULT"
	TestPass       = "PASS"
	TestFail       = "FAIL"
	TestFailReason = "FAIL_REASON"
)

const (
	// The interval and timeout used for polling pod logs.
	interval = 1 * time.Second
	timeout  = 4 * time.Minute
)

// CreatePerfTestCase creates a perf test case with the provided name and value
func CreatePerfTestCase(metricValue float32, metricName, testName string) junit.TestCase {
	tp := []junit.TestProperty{{Name: perfLatency, Value: fmt.Sprintf("%f", metricValue)}}
	tc := junit.TestCase{
		ClassName:  testName,
		Name:       fmt.Sprintf("%s/%s", testName, metricName),
		Properties: junit.TestProperties{Properties: tp}}
	return tc
}

// ParseTestResultFromLog will parse the test result from the pod log
// TODO(Fredy-Z): this is very hacky and error prone, we need to find a better way to get the result.
//                Probably write the logs in JSON format as zipkin tracing.
func ParseTestResultFromLog(client *test.KubeClient, podName, containerName, namespace string) (map[string]string, error) {
	res := make(map[string]string)
	if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		logs, err := client.PodLogs(podName, containerName, namespace)
		if err != nil {
			return true, err
		}
		if strings.Contains(string(logs), TestResultKey) {
			logStr := strings.TrimSpace(string(logs))
			lines := strings.Split(logStr, "\n")
			for i := 1; i < len(lines); i++ {
				keyValue := strings.Split(lines[i], ":")
				res[strings.TrimSpace(keyValue[0])] = strings.TrimSpace(keyValue[1])
			}
			return true, nil
		}
		return false, nil
	}); err != nil {
		return res, err
	}
	return res, nil
}
