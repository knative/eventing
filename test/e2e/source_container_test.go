// +build e2e

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
	"testing"

	"github.com/knative/eventing/test/base"
	"github.com/knative/eventing/test/common"
	"k8s.io/apimachinery/pkg/util/uuid"
)

func TestContainerSource(t *testing.T) {
	const (
		containerSourceName = "e2e-container-source"
		// the heartbeats image is built from test_images/heartbeats
		imageName = "heartbeats"

		loggerPodName = "e2e-container-source-logger-pod"

		saIngressName = "e2e-container-source-ingress"
		crIngressName = "eventing-broker-ingress"
	)
	data := fmt.Sprintf("TestContainerSource%s", uuid.NewUUID())
	// msg is an argument that is used in the heartbeats image
	args := []string{"--msg=" + data}

	client := Setup(t, true)
	defer TearDown(client)

	client.CreateServiceAccountAndBindingOrFail(saIngressName, crIngressName)

	// create event logger pod and service
	loggerPod := base.EventLoggerPod(loggerPodName)
	client.CreatePodOrFail(loggerPod, common.WithService(loggerPodName))

	// create container source
	sinkOption := base.WithSinkServiceForContainerSource(loggerPodName)
	argsOption := base.WithArgsForContainerSource(args)
	saOption := base.WithServiceAccountForContainerSource(saIngressName)
	client.CreateContainerSourceOrFail(containerSourceName, imageName, argsOption, sinkOption, saOption)

	// wait for all test resources to be ready
	if err := client.WaitForAllTestResourcesReady(); err != nil {
		t.Fatalf("Failed to get all test resources ready: %v", err)
	}

	// verify the logger service receives the event
	expectedCount := 2
	if err := client.CheckLog(loggerPodName, common.CheckerContainsAtLeast(data, expectedCount)); err != nil {
		t.Fatalf("String %q does not appear at least %d times in logs of logger pod %q: %v", data, expectedCount, loggerPodName, err)
	}
}
