//go:build e2e
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

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"k8s.io/apimachinery/pkg/util/uuid"
	sourcesv1beta2 "knative.dev/eventing/pkg/apis/sources/v1beta2"
	sugarresources "knative.dev/eventing/pkg/reconciler/sugar/resources"
	rttestingv1beta2 "knative.dev/eventing/pkg/reconciler/testing/v1beta2"
	"knative.dev/eventing/test/e2e/helpers"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestPingSourceV1Beta2EventTypes(t *testing.T) {
	const (
		sourceName = "e2e-ping-source-eventtype"
	)

	client := setup(t, true)
	defer tearDown(client)

	ctx := context.Background()

	// Label namespace so that it creates the default broker.
	if err := client.LabelNamespace(map[string]string{helpers.InjectionLabelKey: helpers.InjectionEnabledLabelValue}); err != nil {
		t.Fatal("Error labeling namespace:", err)
	}

	// Wait for default broker ready.
	client.WaitForResourceReadyOrFail(sugarresources.DefaultBrokerName, testlib.BrokerTypeMeta)

	// Create ping source
	source := rttestingv1beta2.NewPingSource(
		sourceName,
		client.Namespace,
		rttestingv1beta2.WithPingSourceSpec(sourcesv1beta2.PingSourceSpec{
			ContentType: cloudevents.ApplicationJSON,
			Data:        fmt.Sprintf(`{"msg":"TestPingSource %s"}`, uuid.NewUUID()),
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					// TODO change sink to be a non-Broker one once we revisit EventType https://github.com/knative/eventing/issues/2750
					Ref: resources.KnativeRefForBroker(sugarresources.DefaultBrokerName, client.Namespace),
				},
			},
		}),
	)
	client.CreatePingSourceV1Beta2OrFail(source)

	// wait for all test resources to be ready
	client.WaitForAllTestResourcesReadyOrFail(ctx)

	// Verify that an EventType was created.
	eventTypes, err := waitForEventTypes(ctx, client, 1)
	if err != nil {
		t.Fatal("Waiting for EventTypes:", err)
	}
	et := eventTypes[0]
	if et.Spec.Type != sourcesv1beta2.PingSourceEventType && et.Spec.Source.String() != sourcesv1beta2.PingSourceSource(client.Namespace, sourceName) {
		t.Fatalf("Invalid spec.type and/or spec.source for PingSource EventType, expected: type=%s source=%s, got: type=%s source=%s",
			sourcesv1beta2.PingSourceEventType, sourcesv1beta2.PingSourceSource(client.Namespace, sourceName), et.Spec.Type, et.Spec.Source)
	}
}
