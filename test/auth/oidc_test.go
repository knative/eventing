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
	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
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
	env.Test(ctx, t, oidc.BrokerSendEventWithOIDCToken())
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

	env.Test(ctx, t, oidc.AddressableHasAudiencePopulated(channel_impl.GVR(), channel_impl.GVK().Kind, name, env.Namespace()))
}
