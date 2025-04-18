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

	"knative.dev/eventing/test/rekt/features/authz"

	"knative.dev/pkg/system"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"

	"knative.dev/eventing/test/rekt/features/jobsink"
	jsresource "knative.dev/eventing/test/rekt/resources/jobsink"
)

func TestJobSinkSuccess(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.Test(ctx, t, jobsink.Success(""))
}

func TestJobSinkDeleteJobCascadeSecretDeletion(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	jobSinkName := feature.MakeRandomK8sName("jobsink")
	env.Test(ctx, t, jobsink.Success(jobSinkName))
	env.Test(ctx, t, jobsink.DeleteJobsCascadeSecretsDeletion(jobSinkName))
}

func TestJobSinkSuccessTLS(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		eventshub.WithTLS(t),
		environment.Managed(t),
	)

	env.Test(ctx, t, jobsink.SuccessTLS())
}

func TestJobSinkOIDC(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		eventshub.WithTLS(t),
		environment.Managed(t),
	)

	env.Test(ctx, t, jobsink.OIDC())
}

func TestJobSinkSupportsAuthZ(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		eventshub.WithTLS(t),
		environment.Managed(t),
	)

	name := feature.MakeRandomK8sName("jobsink")
	env.Prerequisite(ctx, t, jsresource.GoesReadySimple(name))

	env.TestSet(ctx, t, authz.AddressableAuthZConformance(jsresource.GVR(), "JobSink", name))
}
