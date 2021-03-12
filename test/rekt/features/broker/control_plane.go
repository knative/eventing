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

package broker

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingclientsetv1 "knative.dev/eventing/pkg/client/clientset/versioned/typed/eventing/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/state"
)

func ControlPlaneConformance(brokerName string) *feature.Feature {
	f := new(feature.Feature)

	f.Setup("Set Broker Name", func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, "brokerName", brokerName)
	})

	f.Stable("Control Plane Conformance").
		Should("Broker objects SHOULD include a Ready condition in their status",
			brokerHasReadyInConditions).
		Should("The Broker SHOULD indicate Ready=True when its ingress is available to receive events.",
			readyBrokerHasIngressAvailable).
		Should("While a Broker is Ready, it SHOULD be a valid Addressable and its `status.address.url` field SHOULD indicate the address of its ingress.",
			readyBrokerIsAddressable).
		Should("The class of a Broker object SHOULD be immutable.",
			brokerClassIsImmutable)
	return f
}

func brokerClient(ctx context.Context) eventingclientsetv1.BrokerInterface {
	ec := eventingclient.Get(ctx).EventingV1()
	env := environment.FromContext(ctx)

	return ec.Brokers(env.Namespace())
}
func getBroker(ctx context.Context, t feature.T) *eventingv1.Broker {
	bc := brokerClient(ctx)
	name := state.GetStringOrFail(ctx, t, "brokerName")

	broker, err := bc.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("failed to get broker, %v", err)
	}
	return broker
}

func brokerHasReadyInConditions(ctx context.Context, t feature.T) {
	broker := getBroker(ctx, t)

	if !broker.Status.IsReady() {
		t.Errorf("broker is not ready, %+v", broker.Status.GetTopLevelCondition())
	}
	// Success!
}

func readyBrokerHasIngressAvailable(ctx context.Context, t feature.T) {
	// TODO: I am not sure how to test this from the outside.
}

func readyBrokerIsAddressable(ctx context.Context, t feature.T) {
	broker := getBroker(ctx, t)

	if broker.Status.Address.URL == nil {
		t.Errorf("broker is not addressable")
	}
	// Success!
}

func brokerClassIsImmutable(ctx context.Context, t feature.T) {
	broker := getBroker(ctx, t)

	broker.Annotations[eventingv1.BrokerClassAnnotationKey] = "Rekt.brokerClassIsImmutable"

	if _, err := brokerClient(ctx).Update(ctx, broker, metav1.UpdateOptions{}); err != nil {
		// Success!
		t.Log("broker class is immutable")
	} else {
		t.Errorf("broker class is mutable")
	}
}
