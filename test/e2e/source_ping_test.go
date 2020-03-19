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

	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"

	"k8s.io/apimachinery/pkg/util/uuid"

	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
)

func TestPingSourceV1Alpha1(t *testing.T) {
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
	source := eventingtesting.NewPingSourceV1Alpha1(
		sourceName,
		client.Namespace,
		eventingtesting.WithPingSourceSpec(sourcesv1alpha1.PingSourceSpec{
			Schedule: schedule,
			Data:     data,
			Sink:     &duckv1.Destination{Ref: resources.KnativeRefForService(loggerPodName, client.Namespace)},
		}),
	)
	client.CreatePingSourceV1Alpha1OrFail(source)

	// wait for all test resources to be ready
	client.WaitForAllTestResourcesReadyOrFail()

	// verify the logger service receives the event and only once
	if err := client.CheckLog(loggerPodName, lib.CheckerContainsCount(data, 1)); err != nil {
		t.Fatalf("String %q not found or found multiple times in logs of logger pod %q: %v", data, loggerPodName, err)
	}
}

func TestPingSourceV1Alpha2(t *testing.T) {
	const (
		sourceName = "e2e-ping-source"
		// Every 1 minute starting from now

		loggerPodName = "e2e-ping-source-logger-pod"
	)

	client := setup(t, true)
	defer tearDown(client)

	// create event logger pod and service
	loggerPod := resources.EventLoggerPod(loggerPodName)
	client.CreatePodOrFail(loggerPod, lib.WithService(loggerPodName))

	// create cron job source
	data := fmt.Sprintf("TestPingSource %s", uuid.NewUUID())
	source := eventingtesting.NewPingSourceV1Alpha2(
		sourceName,
		client.Namespace,
		eventingtesting.WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
			JsonData: data,
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: resources.KnativeRefForService(loggerPodName, client.Namespace),
				},
			},
		}),
	)
	client.CreatePingSourceV1Alpha2OrFail(source)

	// wait for all test resources to be ready
	client.WaitForAllTestResourcesReadyOrFail()

	// verify the logger service receives the event and only once
	if err := client.CheckLog(loggerPodName, lib.CheckerContainsCount(data, 1)); err != nil {
		t.Fatalf("String %q not found or found multiple times in logs of logger pod %q: %v", data, loggerPodName, err)
	}
}

func TestPingSourceV1Alpha2ResourceScope(t *testing.T) {
	const (
		sourceName = "e2e-ping-source"
		// Every 1 minute starting from now

		loggerPodName = "e2e-ping-source-logger-pod"
	)

	client := setup(t, true)
	defer tearDown(client)

	// create event logger pod and service
	loggerPod := resources.EventLoggerPod(loggerPodName)
	client.CreatePodOrFail(loggerPod, lib.WithService(loggerPodName))

	// create cron job source
	data := fmt.Sprintf("TestPingSource %s", uuid.NewUUID())
	source := eventingtesting.NewPingSourceV1Alpha2(
		sourceName,
		client.Namespace,
		eventingtesting.WithPingSourceV1A2ResourceScopeAnnotation,
		eventingtesting.WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
			JsonData: data,
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: resources.KnativeRefForService(loggerPodName, client.Namespace),
				},
			},
		}),
	)
	client.CreatePingSourceV1Alpha2OrFail(source)

	// wait for all test resources to be ready
	client.WaitForAllTestResourcesReadyOrFail()

	// verify the logger service receives the event and only once
	if err := client.CheckLog(loggerPodName, lib.CheckerContainsCount(data, 1)); err != nil {
		t.Fatalf("String %q not found or found multiple times in logs of logger pod %q: %v", data, loggerPodName, err)
	}
}
