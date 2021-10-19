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

package experimental

import (
	"testing"

	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/test/experimental/features/delivery_timeout"
	"knative.dev/eventing/test/rekt/features/broker"
	b "knative.dev/eventing/test/rekt/resources/broker"
	rt "knative.dev/eventing/test/rekt/resources/trigger"
	"knative.dev/pkg/system"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"
)

func TestDeliveryTimeout(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.Test(ctx, t, delivery_timeout.ChannelToSink())
}

func TestBrokerTriggerWithDeliveryTimeout(t *testing.T) {
	class := eventing.MTChannelBrokerClassValue

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	brokerName := feature.MakeRandomK8sName("broker")
	triggerName := feature.MakeRandomK8sName("trigger")

	env.Test(ctx, t, broker.GoesReady(brokerName, b.WithBrokerClass(class), b.WithTimeout("PT1S")))
	env.Test(ctx, t, broker.TriggerGoesReady(triggerName, brokerName, rt.WithTimeout("PT10S")))
}
