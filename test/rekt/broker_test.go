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

package rekt

import (
	"testing"
	"time"

	"knative.dev/reconciler-test/pkg/feature"

	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"
	"knative.dev/reconciler-test/pkg/tracing"

	"knative.dev/eventing/test/rekt/features/broker"
	"knative.dev/eventing/test/rekt/features/oidc"
	brokerresources "knative.dev/eventing/test/rekt/resources/broker"
)

func TestBrokerWithManyTriggers(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		tracing.WithGatherer(t),
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
	)

	env.TestSet(ctx, t, broker.ManyTriggers())
}

// TestBrokerWorkFlowWithTransformation test broker transformation respectively follow
// channel flow and trigger event flow.
func TestBrokerWorkFlowWithTransformation(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
	)

	env.TestSet(ctx, t, broker.BrokerWorkFlowWithTransformation())
}

func TestBrokerAsMiddleware(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
	)

	// Install and wait for a Ready Broker.
	env.Prerequisite(ctx, t, broker.GoesReady("default", brokerresources.WithEnvConfig()...))

	// Test that a Broker can act as middleware.
	env.Test(ctx, t, broker.SourceToSink("default"))
	env.Finish()
}

// TestBrokerDLQ
func TestBrokerWithDLQ(t *testing.T) {
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
	)

	// Test that a Broker works as expected with the following topology:
	// source ---> broker --[trigger]--> bad uri
	//                |
	//                +--[DLQ]--> sink
	env.Test(ctx, t, broker.SourceToSinkWithDLQ())
}

// TestBrokerWithFlakyDLQ
func TestBrokerWithFlakyDLQ(t *testing.T) {
	t.Skip("Eventshub needs work")

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
	)

	// Install and wait for a Ready Broker.
	env.Prerequisite(ctx, t, broker.GoesReady("default", brokerresources.WithEnvConfig()...))

	// Test that a Broker can act as middleware.
	env.Test(ctx, t, broker.SourceToSinkWithFlakyDLQ("default"))
}

// TestBrokerConformance
func TestBrokerConformance(t *testing.T) {
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
	)

	// Install and wait for a Ready Broker.
	env.Prerequisite(ctx, t, broker.GoesReady("default", brokerresources.WithEnvConfig()...))
	env.TestSet(ctx, t, broker.DataPlaneConformance("default"))
	env.TestSet(ctx, t, broker.ControlPlaneConformance("default", brokerresources.WithEnvConfig()...))
}

func TestBrokerDefaultDelivery(t *testing.T) {
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
	)

	env.Test(ctx, t, broker.DefaultDeliverySpec())
}

// TestBrokerPreferHeaderCheck test if the test message without explicit prefer header
// should have it after fanout.
func TestBrokerPreferHeaderCheck(t *testing.T) {
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
	)

	env.Test(ctx, t, broker.BrokerPreferHeaderCheck())
}

// TestBrokerRedelivery test Broker reply with a bad status code respectively
// following the fibonacci sequence and Drop first N events
func TestBrokerRedelivery(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
	)

	env.TestSet(ctx, t, broker.BrokerRedelivery())
}

func TestBrokerDeadLetterSinkExtensions(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
	)

	env.TestSet(ctx, t, broker.BrokerDeadLetterSinkExtensions())
}

func TestBrokerDeliverLongMessage(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
	)

	env.TestSet(ctx, t, broker.BrokerDeliverLongMessage())
}

func TestBrokerDeliverLongResponseMessage(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
	)

	env.TestSet(ctx, t, broker.BrokerDeliverLongResponseMessage())
}

func TestMTChannelBrokerRotateTLSCertificates(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		eventshub.WithTLS(t),
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
	)

	env.Test(ctx, t, broker.RotateMTChannelBrokerTLSCertificates())
}

func TestBrokerSupportsOIDC(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		environment.WithPollTimings(4*time.Second, 12*time.Minute),
		eventshub.WithTLS(t),
	)

	name := feature.MakeRandomK8sName("broker")
	env.Prerequisite(ctx, t, broker.GoesReady(name, brokerresources.WithEnvConfig()...))

	env.TestSet(ctx, t, oidc.AddressableOIDCConformance(brokerresources.GVR(), "Broker", name, env.Namespace()))
}

func TestBrokerSendsEventsWithOIDCSupport(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		eventshub.WithTLS(t),
	)

	env.TestSet(ctx, t, broker.BrokerSendEventWithOIDC())
}
