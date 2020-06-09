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
	pkgResources "knative.dev/eventing/pkg/reconciler/mtnamespace/resources"
	"knative.dev/eventing/test/lib/recordevents"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

		loggerPodName = "e2e-ping-source-logger-pod-v1alpha1"
	)

	client := setup(t, true)
	defer tearDown(client)

	// create event logger pod and service
	loggerPod := resources.EventRecordPod(loggerPodName)
	client.CreatePodOrFail(loggerPod, lib.WithService(loggerPodName))
	targetTracker, err := recordevents.NewEventInfoStore(client, loggerPodName)
	if err != nil {
		t.Fatalf("Pod tracker failed: %v", err)
	}
	defer targetTracker.Cleanup()

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
	err = targetTracker.WaitMatchSourceData(sourcesv1alpha1.PingSourceSource(client.Namespace, sourceName), data, 1, 1)
	if err != nil {
		t.Fatalf("Error watching for data %s event in pod %s: %v", data, loggerPodName, err)
	}
}

func TestPingSourceV1Alpha2(t *testing.T) {
	const (
		sourceName = "e2e-ping-source"
		// Every 1 minute starting from now

		loggerPodName = "e2e-ping-source-logger-pod-v1alpha2"
	)

	client := setup(t, true)
	defer tearDown(client)

	// create event logger pod and service
	loggerPod := resources.EventRecordPod(loggerPodName)
	client.CreatePodOrFail(loggerPod, lib.WithService(loggerPodName))
	targetTracker, err := recordevents.NewEventInfoStore(client, loggerPodName)
	if err != nil {
		t.Fatalf("Pod tracker failed: %v", err)
	}
	defer targetTracker.Cleanup()

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
	err = targetTracker.WaitMatchSourceData(sourcesv1alpha2.PingSourceSource(client.Namespace, sourceName), data, 1, 1)
	if err != nil {
		t.Fatalf("Error watching for data %s event in pod %s: %v", data, loggerPodName, err)
	}
}

func TestPingSourceV1Alpha2ResourceScope(t *testing.T) {
	const (
		sourceName = "e2e-ping-source"
		// Every 1 minute starting from now

		loggerPodName = "e2e-ping-source-logger-pod-v1alpha2rs"
	)

	client := setup(t, true)
	defer tearDown(client)

	// create event logger pod and service
	loggerPod := resources.EventRecordPod(loggerPodName)
	client.CreatePodOrFail(loggerPod, lib.WithService(loggerPodName))
	targetTracker, err := recordevents.NewEventInfoStore(client, loggerPodName)
	if err != nil {
		t.Fatalf("Pod tracker failed: %v", err)
	}
	defer targetTracker.Cleanup()

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
	err = targetTracker.WaitMatchSourceData(sourcesv1alpha2.PingSourceSource(client.Namespace, sourceName), data, 1, 1)
	if err != nil {
		t.Fatalf("Error watching for data %s event in pod %s: %v", data, loggerPodName, err)
	}
}

func TestPingSourceV1Alpha2EventTypes(t *testing.T) {
	const (
		sourceName = "e2e-ping-source-eventtype"
	)

	client := setup(t, true)
	defer tearDown(client)

	// Label namespace so that it creates the default broker.
	if err := client.LabelNamespace(map[string]string{"knative-eventing-injection": "enabled"}); err != nil {
		t.Fatalf("Error annotating namespace: %v", err)
	}

	// Wait for default broker ready.
	client.WaitForResourceReadyOrFail(pkgResources.DefaultBrokerName, lib.BrokerTypeMeta)

	// Create ping source
	data := fmt.Sprintf("TestPingSource %s", uuid.NewUUID())
	source := eventingtesting.NewPingSourceV1Alpha2(
		sourceName,
		client.Namespace,
		eventingtesting.WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
			JsonData: data,
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					// TODO change sink to be a non-Broker one once we revisit EventType https://github.com/knative/eventing/issues/2750
					Ref: resources.KnativeRefForBroker(defaultBrokerName, client.Namespace),
				},
			},
		}),
	)
	client.CreatePingSourceV1Alpha2OrFail(source)

	// wait for all test resources to be ready
	client.WaitForAllTestResourcesReadyOrFail()

	// verify that an EventType was created.
	eventTypes, err := client.Eventing.EventingV1beta1().EventTypes(client.Namespace).List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Error retrieving EventTypes: %v", err)
	}
	if len(eventTypes.Items) != 1 {
		t.Fatalf("Invalid number of EventTypes registered for PingSource: %s/%s, expected 1, got %d", client.Namespace, sourceName, len(eventTypes.Items))
	}
	et := eventTypes.Items[0]
	if et.Spec.Type != sourcesv1alpha2.PingSourceEventType && et.Spec.Source.String() != sourcesv1alpha2.PingSourceSource(client.Namespace, sourceName) {
		t.Fatalf("Invalid spec.type and/or spec.source for PingSource EventType, expected: type=%s source=%s, got: type=%s source=%s",
			sourcesv1alpha2.PingSourceEventType, sourcesv1alpha2.PingSourceSource(client.Namespace, sourceName), et.Spec.Type, et.Spec.Source)
	}
}
