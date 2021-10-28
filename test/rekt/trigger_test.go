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

	"knative.dev/eventing/test/rekt/features/broker"
	"knative.dev/eventing/test/rekt/features/trigger"
	b "knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"
)

func TestTriggerDefaulting(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(environment.Managed(t))

	env.TestSet(ctx, t, trigger.Defaulting())

	env.Finish()
}

func TestTriggerWithDLS(t *testing.T) {
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	// The following will reuse the same environment for two different tests.

	prober := eventshub.NewProber()

	// Test that a Trigger DLS "test1" works as expected with the following topology:
	// source ---> broker<Via> --[trigger]--> bad uri
	//                               |
	//                               +--[DLS]--> sink
	// Wait till broker is ready since we need it to run the test
	brokerName := "normal-broker"
	env.Prerequisite(ctx, t, broker.GoesReady(brokerName, b.WithEnvConfig()...))
	env.Test(ctx, t, trigger.SourceToTriggerSinkWithDLS("test1", brokerName, prober))

	// Test that a Trigger DLS "test2" works as expected with the following topology:
	// source ---> broker --[trigger]--> bad uri
	//              |          |
	//              x--[DLS]   +--[DLS]--> sink
	//
	brokerName = "dls-broker"
	brokerSinkName := "broker-sink"
	env.Prerequisite(ctx, t, broker.GoesReadyWithProbeReceiver(brokerName, brokerSinkName, prober, b.WithEnvConfig()...))
	env.Test(ctx, t, trigger.SourceToTriggerSinkWithDLSDontUseBrokers("test2", brokerName, brokerSinkName, prober))
}
