//go:build e2e
// +build e2e

/*
Copyright 2022 The Knative Authors

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

	newfilters "knative.dev/eventing/test/rekt/features/new_trigger_filters"

	"knative.dev/pkg/system"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"

	"knative.dev/eventing/test/rekt/resources/broker"
)

func TestMTChannelBrokerNewTriggerFilters(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)
	brokerName := "default"

	env.Prerequisite(ctx, t, broker.InstallMTBroker(brokerName))
	env.TestSet(ctx, t, newfilters.FiltersFeatureSet(brokerName))
}

func TestMTChannelBrokerAnyTriggerFilters(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)
	brokerName := "default"

	env.Prerequisite(ctx, t, broker.InstallMTBroker(brokerName))
	env.Test(ctx, t, newfilters.AnyFilterFeature(brokerName))
}

func TestMTChannelBrokerAllTriggerFilters(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)
	brokerName := "default"

	env.Prerequisite(ctx, t, broker.InstallMTBroker(brokerName))
	env.Test(ctx, t, newfilters.AllFilterFeature(brokerName))
}
