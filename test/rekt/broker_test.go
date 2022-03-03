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

	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"

	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/test/rekt/features/broker"
	b "knative.dev/eventing/test/rekt/resources/broker"
)

// TestBrokerAsMiddleware
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
	env.Prerequisite(ctx, t, broker.GoesReady("default", b.WithBrokerClass(eventing.MTChannelBrokerClassValue)))

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

	class := eventing.MTChannelBrokerClassValue

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
	)

	// Install and wait for a Ready Broker.
	env.Prerequisite(ctx, t, broker.GoesReady("default", b.WithBrokerClass(class)))

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
	env.Prerequisite(ctx, t, broker.GoesReady("default", b.WithEnvConfig()...))
	env.TestSet(ctx, t, broker.ControlPlaneConformance("default"))
	env.TestSet(ctx, t, broker.DataPlaneConformance("default"))
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
