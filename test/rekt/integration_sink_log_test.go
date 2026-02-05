//go:build e2e
// +build e2e

/*
Copyright 2024 The Knative Authors

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

	"knative.dev/eventing/test/rekt/features/authz"
	"knative.dev/eventing/test/rekt/features/integrationsink"
	"knative.dev/eventing/test/rekt/features/oidc"
	integrationsinkresource "knative.dev/eventing/test/rekt/resources/integrationsink"
	"knative.dev/pkg/system"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"
)

func TestIntegrationSinkLogSuccess(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithObservabilityConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.Test(ctx, t, integrationsink.Success(integrationsinkresource.SinkTypeLog))
}

func TestIntegrationSinkLogSuccessTLS(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithObservabilityConfig,
		k8s.WithEventListener,
		eventshub.WithTLS(t),
		environment.Managed(t),
	)

	env.Test(ctx, t, integrationsink.SuccessTLS(integrationsinkresource.SinkTypeLog))
}

func TestIntegrationSinkLogSupportsOIDC(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithObservabilityConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		environment.WithPollTimings(4*time.Second, 12*time.Minute),
		eventshub.WithTLS(t),
	)

	name := feature.MakeRandomK8sName("integrationsink")
	env.Prerequisite(ctx, t, integrationsinkresource.GoesReadySimple(name))

	env.TestSet(ctx, t, oidc.AddressableOIDCConformance(integrationsinkresource.GVR(), "IntegrationSink", name, env.Namespace()))
}

func TestIntegrationSinkLogSupportsAuthZ(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithObservabilityConfig,
		k8s.WithEventListener,
		eventshub.WithTLS(t),
		environment.Managed(t),
	)

	name := feature.MakeRandomK8sName("integrationsink")
	env.Prerequisite(ctx, t, integrationsinkresource.GoesReadySimple(name))

	env.TestSet(ctx, t, authz.AddressableAuthZConformance(integrationsinkresource.GVR(), "IntegrationSink", name))
}
