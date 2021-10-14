//go:build e2e
// +build e2e

/*
Copyright 2021 The Knative Authors

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

package helpers

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func BrokerDLSStatusUpdate(
	ctx context.Context,
	brokerClass string,
	t *testing.T,
	channel metav1.TypeMeta,
	options ...testlib.SetupClientOption) {

	const (
		brokerName   = "test-broker"
		recorderName = "event-recorder"
	)

	tests := []struct {
		name string
	}{
		{
			name: "broker with deadLetterSink.ref updates its status.deadLetterSinkUri",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := testlib.Setup(t, true)
			defer testlib.TearDown(client)

			recordevents.DeployEventRecordOrFail(ctx, client, recorderName)

			config := client.CreateBrokerConfigMapOrFail("config-"+brokerName, &channel)

			// create a new broker.
			client.CreateBrokerOrFail(
				brokerName,
				resources.WithBrokerClassForBroker(brokerClass),
				resources.WithConfigForBroker(config),
				func(broker *v1.Broker) {
					broker.Spec.Delivery = &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: resources.KnativeRefForService(recorderName, client.Namespace),
						},
					}
				},
			)

			client.WaitForAllTestResourcesReadyOrFail(ctx)

			var br *v1.Broker
			brokers := client.Eventing.EventingV1().Brokers(client.Namespace)
			err := client.RetryWebhookErrors(func(attempts int) (err error) {
				var e error
				client.T.Logf("Getting v1 Broker %s", brokerName)
				br, e = brokers.Get(context.Background(), brokerName, metav1.GetOptions{})
				if e != nil {
					t.Logf("Failed to get Broker %q: %v", brokerName, e)
				}
				return err
			})

			if err != nil {
				t.Fatalf("Error: Could not get broker %s: %v", brokerName, err)
			}

			if br.Status.DeadLetterSinkURI == nil {
				t.Fatalf("Error: broker.Status.DeadLetterSinkURI is nil but resource reported Ready")
			}
		})
	}

}
