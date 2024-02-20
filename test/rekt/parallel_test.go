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

	"knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/reconciler-test/pkg/feature"

	"knative.dev/pkg/system"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"

	"knative.dev/eventing/test/rekt/features/parallel"
	"knative.dev/eventing/test/rekt/resources/channel_template"
	parallelresources "knative.dev/eventing/test/rekt/resources/parallel"
)

func TestParallel(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)
	t.Cleanup(env.Finish)

	env.Test(ctx, t, parallel.ParallelWithTwoBranches(channel_template.ImmemoryChannelTemplate()))
}

func TestParallelTLS(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		eventshub.WithTLS(t),
	)
	t.Cleanup(env.Finish)

	env.Test(ctx, t, parallel.ParallelWithTwoBranchesTLS(channel_template.ImmemoryChannelTemplate()))
}

func TestParallelSupportsOIDC(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		eventshub.WithTLS(t),
	)

	name := feature.MakeRandomK8sName("parallel")
	env.Prerequisite(ctx, t, parallel.GoesReady(name, parallelresources.WithChannelTemplate(channel_template.ChannelTemplate{
		TypeMeta: channel_impl.TypeMeta(),
		Spec:     map[string]interface{}{},
	})))

	env.Test(ctx, t, parallel.ParallelHasAudienceOfInputChannel(name, env.Namespace(), channel_impl.GVR(), channel_impl.GVK().Kind))
}

func TestParallelTwoBranchesWithOIDC(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		eventshub.WithTLS(t),
	)

	env.Test(ctx, t, parallel.ParallelWithTwoBranchesOIDC(channel_template.ImmemoryChannelTemplate()))
}
