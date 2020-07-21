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

package ha

import (
	"fmt"
	"math"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
	"knative.dev/pkg/system"
	"knative.dev/pkg/test/ha"
)

const (
	sourceName         = "e2e-ping-source-ha"
	recordEventPodName = "e2e-ping-source-ha-logger-pod"
	adapterName        = "pingsource-mt-adapter"
)

// TestPingSourceHA checks that when HA is disabled
// events are lost, and when it is enabled, no events are lost
func TestPingSourceHA(t *testing.T) {
	client := setup(t, true)
	defer tearDown(client)

	const interval = 3

	// create event logger pod and service
	eventTracker, _ := recordevents.StartEventRecordOrFail(client, recordEventPodName)
	defer eventTracker.Cleanup()

	t.Logf("creating 1 PingSource")
	data := fmt.Sprintf(`{"msg":"TestPingSource-%s"}`, uuid.NewUUID())
	source := eventingtesting.NewPingSourceV1Alpha2(
		fmt.Sprintf(sourceName),
		client.Namespace,
		eventingtesting.WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
			JsonData: data,
			Schedule: fmt.Sprintf("*/%d * * * * *", interval),
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: resources.KnativeRefForService(recordEventPodName, client.Namespace),
				},
			},
		}),
	)

	client.CreatePingSourceV1Alpha2OrFail(source)

	// wait for all test resources to be ready
	t.Log("waiting for all resources to be ready")
	client.WaitForAllTestResourcesReadyOrFail()

	// First scenario: 1 replica, kill the ping adapter pod => lost events

	t.Log("set number of replicas to 1")
	err := setDeploymentReplicas(client, adapterName, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cutoff := time.Now()

	// Get pingsource adapter pod
	pods, err := client.Kube.Kube.CoreV1().Pods(system.Namespace()).List(metav1.ListOptions{
		LabelSelector: "eventing.knative.dev/source=ping-source-controller",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Make sure we got only one replica.
	if len(pods.Items) != 1 {
		t.Fatalf("unexpected 1 pingsource adapter, got %d", len(pods.Items))
	}

	time.Sleep(2 * time.Second)
	t.Log("killing the one existing adapter pod")
	err = client.Kube.Kube.CoreV1().Pods(system.Namespace()).Delete(pods.Items[0].Name, &metav1.DeleteOptions{
		GracePeriodSeconds: ptr.Int64(10),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(10 * time.Second)

	// Check that the adapter skipped a beat
	t.Log("checking some events have been lost")
	events, _, _, err := eventTracker.Find()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lost, duplicates := analyze(events, interval*time.Second, cutoff)
	if lost == 0 {
		t.Fatalf("expected some events to be lost")
	}
	if duplicates != 0 {
		t.Fatalf("expected no duplicated events (%d)", duplicates)
	}

	// Second scenario: scale to 2 replicas, make sure no events are lost, no duplicates
	events, _, _, err = eventTracker.Find()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cutoff = events[len(events)-1].Event.Time()

	t.Log("setting number of replicas to 2")
	err = setDeploymentReplicas(client, adapterName, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(10 * time.Second)

	events, _, _, err = eventTracker.Find()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lost, duplicates = analyze(events, interval*time.Second, cutoff)
	if lost != 0 {
		t.Fatalf("expected no events to be lost (%d)", lost)
		t.Logf("events: %v", events)
	}
	if duplicates != 0 {
		t.Fatalf("expected no duplicated events (%d)", duplicates)
	}

	// Third scenario: kill the leader and check no events are lost, no duplicates

	podNames, err := ha.GetLeaders(t, client.Kube, adapterName, system.Namespace())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(podNames) != 1 {
		t.Fatalf("expected only one leader pod, got %v", podNames)
	}

	t.Logf("killing adapter pod %s", podNames[0])
	err = client.Kube.Kube.CoreV1().Pods(system.Namespace()).Delete(podNames[0], &metav1.DeleteOptions{
		GracePeriodSeconds: ptr.Int64(30),
	})

	time.Sleep(10 * time.Second)

	events, _, _, err = eventTracker.Find()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lost, duplicates = analyze(events, interval*time.Second, cutoff)
	if lost != 0 {
		t.Fatalf("expected no events to be lost (%d)", lost)
		t.Logf("events: %v", events)
	}
	if duplicates != 0 {
		t.Fatalf("expected no duplicated events (%d)", duplicates)
	}
}

func analyze(events []recordevents.EventInfo, interval time.Duration, cutoff time.Time) (int, int) {
	if len(events) < 2 {
		return 0, 0
	}
	lastEvent := events[0]
	lost := 0
	duplicated := 0
	for i := 1; i < len(events); i++ {
		if events[i].Event.Time().After(cutoff) {
			elapsed := events[i].Event.Time().Sub(lastEvent.Event.Time())

			// Allow +- 200ms margin
			if math.Abs(float64(elapsed.Milliseconds())-float64(interval.Milliseconds())) > 200 {
				lost++
			}

			// 2 events in less than 100ms are considered duplicates
			if elapsed.Milliseconds() < 100 {
				duplicated++
			}
		}
		lastEvent = events[i]
	}
	return lost, duplicated
}
