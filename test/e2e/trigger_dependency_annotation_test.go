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

	"knative.dev/eventing/pkg/apis/eventing"
	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"
	pkgResources "knative.dev/eventing/pkg/reconciler/mtnamespace/resources"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/pkg/apis"
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
		dependencyAnnotation = `{"kind":"PingSource","name":"test-ping-source-annotation","apiVersion":"sources.knative.dev/v1alpha2"}`
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

	// Wait for subscriber to become ready
	client.WaitForAllTestResourcesReadyOrFail()

	// Create triggers.
	client.CreateTriggerOrFailV1Beta1(triggerName,
		resources.WithSubscriberServiceRefForTriggerV1Beta1(subscriberName),
		resources.WithDependencyAnnotationTriggerV1Beta1(dependencyAnnotation),
	)

	jsonData := fmt.Sprintf("Test trigger-annotation %s", uuid.NewUUID())
	pingSource := eventingtesting.NewPingSourceV1Alpha2(
		pingSourceName,
		client.Namespace,
		eventingtesting.WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
			Schedule: schedule,
			JsonData: jsonData,
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{},
			},
		}),
	)
	if brokerClass == eventing.MTChannelBrokerClassValue {
		pingSource.Spec.SourceSpec.Sink.URI = &apis.URL{
			Scheme: "http",
			Host:   "broker-ingress.knative-eventing.svc.cluster.local",
			Path:   fmt.Sprintf("/%s/%s", client.Namespace, defaultBrokerName),
		}
	}

	client.CreatePingSourceV1Alpha2OrFail(pingSource)

	// Trigger should become ready after pingSource was created
	client.WaitForResourceReadyOrFail(triggerName, lib.TriggerTypeMeta)

	if err := client.CheckLog(subscriberName, lib.CheckerContains(jsonData)); err != nil {
		t.Fatalf("Event(s) not found in logs of subscriber pod %q: %v", subscriberName, err)
	}
}
