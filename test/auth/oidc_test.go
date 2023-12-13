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

package auth

import (
	"testing"
	"time"

	"knative.dev/pkg/system"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"

	"knative.dev/eventing/test/auth/features/oidc"
	brokerfeatures "knative.dev/eventing/test/rekt/features/broker"
	"knative.dev/eventing/test/rekt/features/channel"
	parallelfeatures "knative.dev/eventing/test/rekt/features/parallel"
	sequencefeatures "knative.dev/eventing/test/rekt/features/sequence"
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/eventing/test/rekt/resources/channel_template"
	"knative.dev/eventing/test/rekt/resources/parallel"
	"knative.dev/eventing/test/rekt/resources/sequence"
)

func TestBrokerSupportsOIDC(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		environment.WithPollTimings(4*time.Second, 12*time.Minute),
	)

	name := feature.MakeRandomK8sName("broker")
	env.Prerequisite(ctx, t, brokerfeatures.GoesReady(name, broker.WithEnvConfig()...))

	env.TestSet(ctx, t, oidc.AddressableOIDCConformance(broker.GVR(), "Broker", name, env.Namespace()))
}

func TestBrokerSendsEventsWithOIDCSupport(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.TestSet(ctx, t, oidc.BrokerSendEventWithOIDC())
}

func TestChannelImplSupportsOIDC(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		environment.WithPollTimings(4*time.Second, 12*time.Minute),
	)

	name := feature.MakeRandomK8sName("channelimpl")
	env.Prerequisite(ctx, t, channel.ImplGoesReady(name))

	env.TestSet(ctx, t, oidc.AddressableOIDCConformance(channel_impl.GVR(), channel_impl.GVK().Kind, name, env.Namespace()))
}

func TestParallelSupportsOIDC(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	name := feature.MakeRandomK8sName("parallel")
	env.Prerequisite(ctx, t, parallelfeatures.GoesReady(name, parallel.WithChannelTemplate(channel_template.ChannelTemplate{
		TypeMeta: channel_impl.TypeMeta(),
		Spec:     map[string]interface{}{},
	})))

	env.Test(ctx, t, oidc.ParallelHasAudienceOfInputChannel(name, env.Namespace(), channel_impl.GVR(), channel_impl.GVK().Kind))
}

func TestChannelDispatcherAuthenticatesWithOIDC(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.Test(ctx, t, oidc.ChannelDispatcherAuthenticatesRequestsWithOIDC())
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
	env.Prerequisite(ctx, t, sequencefeatures.GoesReady(name, sequence.WithChannelTemplate(channel_template.ChannelTemplate{
		TypeMeta: channel_impl.TypeMeta(),
		Spec:     map[string]interface{}{},
	})))

	env.Test(ctx, t, oidc.SequenceHasAudienceOfInputChannel(name, env.Namespace(), channel_impl.GVR(), channel_impl.GVK().Kind))
}

func TestContainerSourceSendsEventsWithOIDCSupport(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.Test(ctx, t, oidc.SendsEventsWithSinkRefOIDC())
}

func TestSequenceSendsEventsWithOIDCSupport(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.TestSet(ctx, t, oidc.SequenceSendsEventWithOIDC())
}
