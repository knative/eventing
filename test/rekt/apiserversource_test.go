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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/system"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"

	apiserversourcefeatures "knative.dev/eventing/test/rekt/features/apiserversource"
	"knative.dev/eventing/test/rekt/features/workloads"

	_ "knative.dev/pkg/system/testing"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
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

func TestApiServerSourceDataPlaneLabels(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	// Keep the name of the source short otherwise partial name match won't work because
	// the api apiserversource reconciler logic uses prefix, name and UUID to generate a name,
	// and it cuts the name if all of that is too long.
	srcname := feature.MakeRandomK8sName("src")

	env.Prerequisite(ctx, t, apiserversourcefeatures.Install(srcname))
	env.Prerequisite(ctx, t, apiserversourcefeatures.GoesReady(srcname))

	env.Test(ctx, t, workloads.Selector(workloads.Workload{
		GVR: schema.GroupVersionResource{
			Version:  "v1",
			Resource: "pods",
		},
		PartialName: srcname,
		Selector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/name":      "knative-eventing",
				"app.kubernetes.io/component": "apiserversource-adapter",
			},
		},
	}))
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
