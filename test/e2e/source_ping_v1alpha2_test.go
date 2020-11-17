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
	"context"
	"fmt"
	"testing"

	rttestingv1alpha2 "knative.dev/eventing/pkg/reconciler/testing/v1alpha2"

	"knative.dev/eventing/pkg/reconciler/sugar"

	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"
	sugarresources "knative.dev/eventing/pkg/reconciler/sugar/resources"
	"knative.dev/eventing/test/lib/recordevents"

	. "github.com/cloudevents/sdk-go/v2/test"
	"k8s.io/apimachinery/pkg/util/uuid"

	duckv1 "knative.dev/pkg/apis/duck/v1"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
)

func TestPingSourceV1Alpha2(t *testing.T) {
	const (
		sourceName = "e2e-ping-source"
		// Every 1 minute starting from now

		recordEventPodName = "e2e-ping-source-logger-pod-v1alpha2"
	)

	client := setup(t, true)
	defer tearDown(client)

	ctx := context.Background()

	// create event logger pod and service
	eventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, recordEventPodName)
	// create cron job source
	data := fmt.Sprintf(`{"msg":"TestPingSource %s"}`, uuid.NewUUID())
	source := rttestingv1alpha2.NewPingSource(
		sourceName,
		client.Namespace,
		rttestingv1alpha2.WithPingSourceSpec(sourcesv1alpha2.PingSourceSpec{
			JsonData: data,
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: resources.KnativeRefForService(recordEventPodName, client.Namespace),
				},
			},
		}),
	)
	client.CreatePingSourceV1Alpha2OrFail(source)

	// wait for all test resources to be ready
	client.WaitForAllTestResourcesReadyOrFail(ctx)

	// verify the logger service receives the event and only once
	eventTracker.AssertExact(1, recordevents.MatchEvent(
		HasSource(sourcesv1alpha2.PingSourceSource(client.Namespace, sourceName)),
		HasData([]byte(data)),
	))
}

func TestPingSourceV1Alpha2EventTypes(t *testing.T) {
	const (
		sourceName = "e2e-ping-source-eventtype"
	)

	client := setup(t, true)
	defer tearDown(client)

	ctx := context.Background()

	// Label namespace so that it creates the default broker.
	if err := client.LabelNamespace(map[string]string{sugar.InjectionLabelKey: sugar.InjectionEnabledLabelValue}); err != nil {
		t.Fatal("Error annotating namespace:", err)
	}

	// Wait for default broker ready.
	client.WaitForResourceReadyOrFail(sugarresources.DefaultBrokerName, testlib.BrokerTypeMeta)

	// Create ping source
	source := rttestingv1alpha2.NewPingSource(
		sourceName,
		client.Namespace,
		rttestingv1alpha2.WithPingSourceSpec(sourcesv1alpha2.PingSourceSpec{
			JsonData: fmt.Sprintf(`{"msg":"TestPingSource %s"}`, uuid.NewUUID()),
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					// TODO change sink to be a non-Broker one once we revisit EventType https://github.com/knative/eventing/issues/2750
					Ref: resources.KnativeRefForBroker(sugarresources.DefaultBrokerName, client.Namespace),
				},
			},
		}),
	)
	client.CreatePingSourceV1Alpha2OrFail(source)

	// wait for all test resources to be ready
	client.WaitForAllTestResourcesReadyOrFail(ctx)

	// Verify that an EventType was created.
	eventTypes, err := waitForEventTypes(ctx, client, 1)
	if err != nil {
		t.Fatal("Waiting for EventTypes:", err)
	}
	et := eventTypes[0]
	if et.Spec.Type != sourcesv1alpha2.PingSourceEventType && et.Spec.Source.String() != sourcesv1alpha2.PingSourceSource(client.Namespace, sourceName) {
		t.Fatalf("Invalid spec.type and/or spec.source for PingSource EventType, expected: type=%s source=%s, got: type=%s source=%s",
			sourcesv1alpha2.PingSourceEventType, sourcesv1alpha2.PingSourceSource(client.Namespace, sourceName), et.Spec.Type, et.Spec.Source)
	}
}
