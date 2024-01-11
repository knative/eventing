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

package rekt

import (
	"testing"
	"time"

	"knative.dev/pkg/system"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"

	_ "knative.dev/pkg/system/testing"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"

	apiserversourcefeatures "knative.dev/eventing/test/rekt/features/apiserversource"
)

// TestApiServerSourceValidationWebhookConfigurationOnCreate tests if the webhook
// is configured correctly for ApiServerSource validation on resource creation.
func TestApiServerSourceValidationWebhookConfigurationOnCreate(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(environment.Managed(t))

	env.Test(ctx, t, apiserversourcefeatures.CreateWithInvalidSpec())
}

// TestApiServerSourceValidationWebhookConfigurationOnUpdate tests if the webhook
// is configured correctly for ApiServerSource validation on resource update.
func TestApiServerSourceValidationWebhookConfigurationOnUpdate(t *testing.T) {
	t.Parallel()

	srcname := feature.MakeRandomK8sName("apiserversource")

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.Prerequisite(ctx, t, apiserversourcefeatures.Install(srcname))
	env.Prerequisite(ctx, t, apiserversourcefeatures.GoesReady(srcname))

	env.Test(ctx, t, apiserversourcefeatures.UpdateWithInvalidSpec(srcname))
}

func TestApiServerSourceDataPlane_SinkTypes(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		environment.WithPollTimings(5*time.Second, 2*time.Minute),
	)

	env.TestSet(ctx, t, apiserversourcefeatures.DataPlane_SinkTypes())
}

func TestApiServerSourceDataPlane_BrokerAsSinkTLS(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		eventshub.WithTLS(t),
		environment.WithPollTimings(5*time.Second, 2*time.Minute),
	)

	env.Test(ctx, t, apiserversourcefeatures.SendsEventsWithBrokerAsSinkTLS())
}

func TestApiServerSourceDataPlaneTLS(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		//environment.Managed(t),
		eventshub.WithTLS(t),
	)

	env.ParallelTest(ctx, t, apiserversourcefeatures.SendsEventsWithTLS())
	env.ParallelTest(ctx, t, apiserversourcefeatures.SendsEventsWithTLSTrustBundle())
}

func TestApiServerSourceDataPlane_EventModes(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.TestSet(ctx, t, apiserversourcefeatures.DataPlane_EventModes())
}

func TestApiServerSourceDataPlane_ResourceMatching(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.TestSet(ctx, t, apiserversourcefeatures.DataPlane_ResourceMatching())
}

func TestApiServerSourceDataPlane_EventsRetries(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.Test(ctx, t, apiserversourcefeatures.SendsEventsWithRetries())
}

func TestApiServerSourceDataPlane_MultipleNamespaces(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.Test(ctx, t, apiserversourcefeatures.SendsEventsForAllResourcesWithNamespaceSelector())
}

func TestApiServerSourceDataPlane_MultipleNamespacesEmptySelector(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.Test(ctx, t, apiserversourcefeatures.SendsEventsForAllResourcesWithEmptyNamespaceSelector())
}
