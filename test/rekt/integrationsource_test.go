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
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"

	"knative.dev/eventing/test/rekt/features/integrationsource"
)

func TestIntegrationSourceWithSinkRef(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithObservabilityConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
	)
	t.Cleanup(env.Finish)

	env.Test(ctx, t, integrationsource.SendsEventsWithSinkRef())
}

func TestIntegrationSourceWithTLS(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithObservabilityConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		eventshub.WithTLS(t),
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
	)

	env.ParallelTest(ctx, t, integrationsource.SendEventsWithTLSRecieverAsSink())
	env.ParallelTest(ctx, t, integrationsource.SendEventsWithTLSRecieverAsSinkTrustBundle())
}

func TestIntegrationSourceSendsEventsWithOIDC(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithObservabilityConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		eventshub.WithTLS(t),
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
	)

	env.Test(ctx, t, integrationsource.SendsEventsWithSinkRefOIDC())
}
