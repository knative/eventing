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
	duckv1 "knative.dev/pkg/apis/duck/v1"

	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	pkgResources "knative.dev/eventing/pkg/reconciler/namespace/resources"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
)

// This test is for avoiding regressions on the trigger dependency annotation functionality.
// It will first create a trigger with the dependency annotation, and then create a pingSource.
// Broker controller should make trigger become ready after pingSource is ready.
// This trigger dependency annotation is related on issue #1734.
func TestTriggerDependencyAnnotation(t *testing.T) {
	const (
		defaultBrokerName    = pkgResources.DefaultBrokerName
		triggerName          = "trigger-annotation"
		subscriberName       = "subscriber-annotation"
		dependencyAnnotation = `{"kind":"PingSource","name":"test-ping-source-annotation","apiVersion":"sources.knative.dev/v1alpha1"}`
		pingSourceName       = "test-ping-source-annotation"
		// Every 1 minute starting from now
		schedule = "*/1 * * * *"
	)
	client := setup(t, true)
	defer tearDown(client)

	// Label namespace so that it creates the default broker.
	if err := client.LabelNamespace(map[string]string{"knative-eventing-injection": "enabled"}); err != nil {
		t.Fatalf("Error annotating namespace: %v", err)
	}
	// Wait for default broker ready.
	client.WaitForResourceReadyOrFail(defaultBrokerName, lib.BrokerTypeMeta)

	// Create subscribers.
	pod := resources.EventLoggerPod(subscriberName)
	client.CreatePodOrFail(pod, lib.WithService(subscriberName))

	// Create triggers.
	client.CreateTriggerOrFail(triggerName,
		resources.WithSubscriberServiceRefForTrigger(subscriberName),
		resources.WithDependencyAnnotaionTrigger(dependencyAnnotation),
	)

	data := fmt.Sprintf("Test trigger-annotation %s", uuid.NewUUID())
	pingSource := eventingtesting.NewPingSourceV1Alpha1(
		pingSourceName,
		client.Namespace,
		eventingtesting.WithPingSourceSpec(sourcesv1alpha1.PingSourceSpec{
			Schedule: schedule,
			Data:     data,
			Sink:     &duckv1.Destination{Ref: resources.KnativeRefForService(defaultBrokerName+"-broker", client.Namespace)},
		}),
	)
	client.CreatePingSourceV1Alpha1OrFail(pingSource)

	// Trigger should become ready after pingSource was created
	client.WaitForResourceReadyOrFail(triggerName, lib.TriggerTypeMeta)

	if err := client.CheckLog(subscriberName, lib.CheckerContains(data)); err != nil {
		t.Fatalf("Event(s) not found in logs of subscriber pod %q: %v", subscriberName, err)
	}
}
