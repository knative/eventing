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
	"knative.dev/reconciler-test/pkg/feature"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/binding"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/tracing"

	"knative.dev/eventing/test/rekt/features/channel"
	"knative.dev/eventing/test/rekt/features/oidc"
	ch "knative.dev/eventing/test/rekt/resources/channel"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/eventing/test/rekt/resources/subscription"
)

// TestChannelConformance
func TestChannelConformance(t *testing.T) {
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
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
			subscription.WithSubscriber(nil, "http://example.com", "")),
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
	chRef := channel_impl.AsRef(channelName)

	names := []string{
		"customname",
		"name-with-dash",
		"name1with2numbers3",
		"name63-01234567890123456789012345678901234567890123456789012345",
	}

	for _, name := range names {
		env.Test(ctx, t, channel.SubscriptionGoesReady(name,
			subscription.WithChannel(chRef),
			subscription.WithSubscriber(nil, "http://example.com", "")),
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

	createSubscriberFn := func(ref *duckv1.KReference, uri string) manifest.CfgFn {
		return subscription.WithSubscriber(ref, uri, "")
	}
	env.Test(ctx, t, channel.ChannelChain(10, createSubscriberFn))
}

/*
TestChannelDeadLetterSink tests if the events that cannot be delivered end up in
the dead letter sink.

It uses Subscription's spec.reply as spec.subscriber.
*/
func TestChannelDeadLetterSink(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	createSubscriberFn := func(ref *duckv1.KReference, uri string) manifest.CfgFn {
		return subscription.WithSubscriber(ref, uri, "")
	}
	env.Test(ctx, t, channel.DeadLetterSink(createSubscriberFn))
}

// TestGenericChannelDeadLetterSink tests if the events that cannot be delivered end up in
// the dead letter sink.
func TestGenericChannelDeadLetterSink(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	createSubscriberFn := func(ref *duckv1.KReference, uri string) manifest.CfgFn {
		return subscription.WithSubscriber(ref, uri, "")
	}
	env.Test(ctx, t, channel.DeadLetterSinkGenericChannel(createSubscriberFn))
	env.Test(ctx, t, channel.AsDeadLetterSink(createSubscriberFn))
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

// TestChannelPreferHeaderCheck test if the test message without explicit prefer header
// should have it after fanout.
func TestChannelPreferHeaderCheck(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	createSubscriberFn := func(ref *duckv1.KReference, uri string) manifest.CfgFn {
		return subscription.WithSubscriber(ref, uri, "")
	}

	env.Test(ctx, t, channel.ChannelPreferHeaderCheck(createSubscriberFn))
}

func TestChannelDeadLetterSinkExtensions(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		tracing.WithGatherer(t),
		environment.Managed(t),
	)

	createSubscriberFn := func(ref *duckv1.KReference, uri string) manifest.CfgFn {
		return subscription.WithSubscriber(ref, uri, "")
	}

	env.TestSet(ctx, t, channel.ChannelDeadLetterSinkExtensions(createSubscriberFn))
}

func TestInMemoryChannelRotateIngressTLSCertificate(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		eventshub.WithTLS(t),
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
	)

	env.Test(ctx, t, channel.RotateDispatcherTLSCertificate())
}

func TestInMemoryChannelTLS(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		eventshub.WithTLS(t),
		environment.WithPollTimings(5*time.Second, 4*time.Minute),
	)

	env.ParallelTest(ctx, t, channel.SubscriptionTLS())
	env.ParallelTest(ctx, t, channel.SubscriptionTLSTrustBundle())
}

func TestChannelImplDispatcherAuthenticatesWithOIDC(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
		eventshub.WithTLS(t),
	)

	env.Test(ctx, t, channel.DispatcherAuthenticatesRequestsWithOIDC())
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
