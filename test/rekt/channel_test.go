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

	"github.com/cloudevents/sdk-go/v2/binding"
	"knative.dev/eventing/test/rekt/features/channel"
	ch "knative.dev/eventing/test/rekt/resources/channel"
	chimpl "knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/eventing/test/rekt/resources/subscription"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"
)

// TestChannelConformance
func TestChannelConformance(t *testing.T) {
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	channelName := "mychannelimpl"

	// Install and wait for a Ready Channel.
	env.Prerequisite(ctx, t, channel.ImplGoesReady(channelName))

	env.TestSet(ctx, t, channel.ControlPlaneConformance(channelName))
	env.TestSet(ctx, t, channel.DataPlaneConformance(channelName))
}

// TestSmoke_Channel
func TestSmoke_Channel(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment()
	t.Cleanup(env.Finish)

	names := []string{
		"customname",
		"name-with-dash",
		"name1with2numbers3",
		"name63-01234567890123456789012345678901234567890123456789012345",
	}

	for _, name := range names {
		env.Test(ctx, t, channel.GoesReady(name))
	}
}

// TestSmoke_ChannelImpl
func TestSmoke_ChannelImpl(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment()
	t.Cleanup(env.Finish)

	names := []string{
		"customname",
		"name-with-dash",
		"name1with2numbers3",
		"name63-01234567890123456789012345678901234567890123456789012345",
	}

	for _, name := range names {
		env.Test(ctx, t, channel.ImplGoesReady(name))
	}

}

// TestSmoke_ChannelWithSubscription
func TestSmoke_ChannelWithSubscription(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment()
	t.Cleanup(env.Finish)

	channelName := "mychannel"

	// Install and wait for a Ready Channel.
	env.Prerequisite(ctx, t, channel.GoesReady(channelName))
	chRef := ch.AsRef(channelName)

	names := []string{
		"customname",
		"name-with-dash",
		"name1with2numbers3",
		"name63-01234567890123456789012345678901234567890123456789012345",
	}

	for _, name := range names {
		env.Test(ctx, t, channel.SubscriptionGoesReady(name,
			subscription.WithChannel(chRef),
			subscription.WithSubscriber(nil, "http://example.com")),
		)
	}
}

// TestSmoke_ChannelImplWithSubscription
func TestSmoke_ChannelImplWithSubscription(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment()
	t.Cleanup(env.Finish)

	channelName := "mychannelimpl"

	// Install and wait for a Ready Channel.
	env.Prerequisite(ctx, t, channel.ImplGoesReady(channelName))
	chRef := chimpl.AsRef(channelName)

	names := []string{
		"customname",
		"name-with-dash",
		"name1with2numbers3",
		"name63-01234567890123456789012345678901234567890123456789012345",
	}

	for _, name := range names {
		env.Test(ctx, t, channel.SubscriptionGoesReady(name,
			subscription.WithChannel(chRef),
			subscription.WithSubscriber(nil, "http://example.com")),
		)
	}
}

/*
TestChannelChain tests the following scenario:

EventSource ---> (Channel ---> Subscription) x 10 ---> Sink

*/
func TestChannelChain(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.Test(ctx, t, channel.ChannelChain(10))
}

func TestChannelDeadLetterSink(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.Test(ctx, t, channel.DeadLetterSink())
}

/*
TestEventTransformationForSubscription tests the following scenario:

             1            2                 5            6                  7
EventSource ---> Channel ---> Subscription ---> Channel ---> Subscription ----> Service(Logger)
                                   |  ^
                                 3 |  | 4
                                   |  |
                                   |  ---------
                                   -----------> Service(Transformation)
*/
func TestEventTransformationForSubscriptionV1(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.Test(ctx, t, channel.EventTransformation())
}

/*
TestBinaryEventForChannel tests the following scenario:

EventSource (binary-encoded messages) ---> Channel ---> Subscription ---> Service(Logger)

*/
func TestBinaryEventForChannel(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.Test(ctx, t, channel.SingleEventWithEncoding(binding.EncodingBinary))
}

/*
TestStructuredEventForChannel tests the following scenario:

EventSource (structured-encoded messages) ---> Channel ---> Subscription ---> Service(Logger)

*/
func TestStructuredEventForChannel(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	env.Test(ctx, t, channel.SingleEventWithEncoding(binding.EncodingStructured))
}
