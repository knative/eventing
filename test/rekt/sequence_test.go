//go:build e2e
// +build e2e

/*
Copyright 2023 The Knative Authors

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

	"knative.dev/reconciler-test/pkg/feature"

	"knative.dev/eventing/test/rekt/features/sequence"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/eventing/test/rekt/resources/channel_template"
	sequenceresources "knative.dev/eventing/test/rekt/resources/sequence"
	"knative.dev/pkg/system"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"
)

func TestSequence(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)
	t.Cleanup(env.Finish)

	env.Test(ctx, t, sequence.SequenceTest(channel_template.ImmemoryChannelTemplate()))
}

func TestSequenceTLS(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		eventshub.WithTLS(t),
	)

	env.Test(ctx, t, sequence.SequenceTestTLS(channel_template.ImmemoryChannelTemplate()))
}

func TestSequenceSupportsOIDC(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	name := feature.MakeRandomK8sName("sequence")
	env.Prerequisite(ctx, t, sequence.GoesReady(name, sequenceresources.WithChannelTemplate(channel_template.ChannelTemplate{
		TypeMeta: channel_impl.TypeMeta(),
		Spec:     map[string]interface{}{},
	})))

	env.Test(ctx, t, sequence.SequenceHasAudienceOfInputChannel(name, env.Namespace(), channel_impl.GVR(), channel_impl.GVK().Kind))
}

func TestSequenceSendsEventsOIDC(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		eventshub.WithTLS(t),
	)

	env.TestSet(ctx, t, sequence.SequenceSendsEventWithOIDC())
}
