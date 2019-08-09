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

	"k8s.io/apimachinery/pkg/util/uuid"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"
)

func TestContainerSource(t *testing.T) {
	const (
		containerSourceName = "e2e-container-source"
		templateName        = "e2e-container-source-template"
		// the heartbeats image is built from test_images/heartbeats
		imageName = "heartbeats"

		loggerPodName = "e2e-container-source-logger-pod"
	)

	client := setup(t, true)
	defer tearDown(client)

	// create event logger pod and service
	loggerPod := resources.EventLoggerPod(loggerPodName)
	client.CreatePodOrFail(loggerPod, common.WithService(loggerPodName))

	data := fmt.Sprintf("TestContainerSource%s", uuid.NewUUID())
	// args are the arguments passing to the container, msg is used in the heartbeats image
	args := []string{"--msg=" + data}

	// create container source
	template := resources.ContainerSourceBasicTemplate(templateName, client.Namespace, imageName, args)
	templateOption := resources.WithTemplateForContainerSource(template)
	sinkOption := resources.WithSinkServiceForContainerSource(loggerPodName)
	client.CreateContainerSourceOrFail(containerSourceName, templateOption, sinkOption)

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
