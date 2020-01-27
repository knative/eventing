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

package e2e

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/util/uuid"

	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
)

func TestPingSource(t *testing.T) {
	const (
		sourceName = "e2e-ping-source"
		// Every 1 minute starting from now
		schedule = "*/1 * * * *"

		loggerPodName = "e2e-ping-source-logger-pod"
	)

	client := setup(t, true)
	defer tearDown(client)

	// create event logger pod and service
	loggerPod := resources.EventLoggerPod(loggerPodName)
	client.CreatePodOrFail(loggerPod, lib.WithService(loggerPodName))

	// create cron job source
	data := fmt.Sprintf("TestPingSource %s", uuid.NewUUID())
	source := eventingtesting.NewPingSource(
		sourceName,
		client.Namespace,
		eventingtesting.WithPingSourceSpec(sourcesv1alpha1.PingSourceSpec{
			Schedule: schedule,
			Data:     data,
			Sink:     &duckv1.Destination{Ref: resources.ServiceRef(loggerPodName)},
		}),
	)
	client.CreatePingSourceOrFail(source)

	// wait for all test resources to be ready
	if err := client.WaitForAllTestResourcesReady(); err != nil {
		t.Fatalf("Failed to get all test resources ready: %v", err)
	}

	// verify the logger service receives the event
	if err := client.CheckLog(loggerPodName, lib.CheckerContains(data)); err != nil {
		t.Fatalf("String %q not found in logs of logger pod %q: %v", data, loggerPodName, err)
	}
}
