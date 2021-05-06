// +build e2e

/*
 * Copyright 2020 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package e2e

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/test/e2e/helpers"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
)

// ChannelBasedBrokerCreator creates a BrokerCreator that creates a broker based on the channel parameter.
func ChannelBasedBrokerCreator(channel metav1.TypeMeta, brokerClass string) helpers.BrokerCreatorWithRetries {
	return func(client *testlib.Client, numRetries int32) string {
		brokerName := strings.ToLower(channel.Kind)

		// create a ConfigMap used by the broker.
		config := client.CreateBrokerConfigMapOrFail("config-"+brokerName, &channel)

		backoff := eventingduckv1.BackoffPolicyLinear

		// create a new broker.
		client.CreateBrokerOrFail(brokerName,
			resources.WithBrokerClassForBroker(brokerClass),
			resources.WithConfigForBroker(config),
			func(broker *v1.Broker) {
				broker.Spec.Delivery = &eventingduckv1.DeliverySpec{
					Retry:         &numRetries,
					BackoffPolicy: &backoff,
					BackoffDelay:  pointer.StringPtr("PT1S"),
				}
			},
		)

		return brokerName
	}
}

func TestBrokerRedelivery(t *testing.T) {

	channelTestRunner.RunTests(t, testlib.FeatureRedelivery, func(t *testing.T, component metav1.TypeMeta) {

		brokerCreator := ChannelBasedBrokerCreator(component, brokerClass)

		helpers.BrokerRedelivery(context.Background(), t, brokerCreator)
	})
}
