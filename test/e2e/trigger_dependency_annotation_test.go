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

	"knative.dev/eventing/pkg/reconciler/sugar"

	. "github.com/cloudevents/sdk-go/v2/test"
	"k8s.io/apimachinery/pkg/util/uuid"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	sourcesv1beta2 "knative.dev/eventing/pkg/apis/sources/v1beta2"
	sugarresources "knative.dev/eventing/pkg/reconciler/sugar/resources"
	eventingtestingv1beta2 "knative.dev/eventing/pkg/reconciler/testing/v1beta2"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
)

// This test is for avoiding regressions on the trigger dependency annotation functionality.
// It will first create a trigger with the dependency annotation, and then create a pingSource.
// Broker controller should make trigger become ready after pingSource is ready.
// This trigger dependency annotation is related on issue #1734.
func TestTriggerDependencyAnnotation(t *testing.T) {
	const (
		defaultBrokerName    = sugarresources.DefaultBrokerName
		triggerName          = "trigger-annotation"
		subscriberName       = "subscriber-annotation"
		dependencyAnnotation = `{"kind":"PingSource","name":"test-ping-source-annotation","apiVersion":"sources.knative.dev/v1beta2"}`
		pingSourceName       = "test-ping-source-annotation"
		// Every 1 minute starting from now
		schedule = "*/1 * * * *"
	)
	client := setup(t, true)
	defer tearDown(client)

	ctx := context.Background()

	// Label namespace so that it creates the default broker.
	if err := client.LabelNamespace(map[string]string{sugar.InjectionLabelKey: sugar.InjectionEnabledLabelValue}); err != nil {
		t.Fatal("Error annotating namespace:", err)
	}
	// Wait for default broker ready.
	client.WaitForResourceReadyOrFail(defaultBrokerName, testlib.BrokerTypeMeta)

	// Create subscribers.
	eventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, subscriberName)
	// Wait for subscriber to become ready
	client.WaitForAllTestResourcesReadyOrFail(ctx)

	// Create triggers.
	client.CreateTriggerOrFail(triggerName,
		resources.WithSubscriberServiceRefForTrigger(subscriberName),
		resources.WithDependencyAnnotationTrigger(dependencyAnnotation),
		resources.WithBroker(defaultBrokerName),
	)

	jsonData := fmt.Sprintf(`{"msg":"Test trigger-annotation %s"}`, uuid.NewUUID())
	pingSource := eventingtestingv1beta2.NewPingSource(
		pingSourceName,
		client.Namespace,
		eventingtestingv1beta2.WithPingSourceSpec(sourcesv1beta2.PingSourceSpec{
			Schedule:    schedule,
			ContentType: cloudevents.ApplicationJSON,
			Data:        jsonData,
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						Namespace:  client.Namespace,
						Name:       defaultBrokerName,
						APIVersion: "eventing.knative.dev/v1",
						Kind:       "Broker",
					},
				},
			},
		}),
	)

	client.CreatePingSourceV1Beta2OrFail(pingSource)

	// Trigger should become ready after pingSource was created
	client.WaitForResourceReadyOrFail(triggerName, testlib.TriggerTypeMeta)

	eventTracker.AssertAtLeast(1, recordevents.MatchEvent(
		HasSource(sourcesv1beta2.PingSourceSource(client.Namespace, pingSourceName)),
		HasData([]byte(jsonData)),
	))
}
