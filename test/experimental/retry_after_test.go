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

package experimental

import (
	"testing"

	"knative.dev/pkg/system"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"
	"knative.dev/reconciler-test/pkg/state"

	"knative.dev/eventing/test/experimental/features/retry_after"
)

func TestRetryAfter(t *testing.T) {

	// Run Test In Parallel With Others
	t.Parallel()

	// Create The Test Context / Environment
	ctx, env := global.Environment(
		knative.WithKnativeNamespace(system.Namespace()),
		knative.WithLoggingConfig,
		knative.WithTracingConfig,
		k8s.WithEventListener,
		environment.Managed(t),
	)

	// Generate A Unique K8S Safe Prefix For The Test Components
	retryAfterPrefix := feature.MakeRandomK8sName("retryafter")

	// Generate Unique Component Names And Add To Context Store
	ctx = state.ContextWith(ctx, &state.KVStore{})
	state.SetOrFail(ctx, t, retry_after.ChannelNameKey, retryAfterPrefix+"-channel")
	state.SetOrFail(ctx, t, retry_after.SubscriptionNameKey, retryAfterPrefix+"-subscription")
	state.SetOrFail(ctx, t, retry_after.SenderNameKey, retryAfterPrefix+"-sender")
	state.SetOrFail(ctx, t, retry_after.ReceiverNameKey, retryAfterPrefix+"-receiver")
	state.SetOrFail(ctx, t, retry_after.RetryAttemptsKey, 3)

	// TestRetryAfter is highly dependent on very low clock drift values
	// among the test involved nodes. If this test fails even with low
	// clock drifts, RetryAfterSecondsKey and ExpectedIntervalMargingKey should
	// be increased.

	// RetryAfterSecondsKey returned Retry-after header, it is expected
	// that a failed event is re-sent just after that duration value.
	//
	// ExpectedIntervalMargingKey is the acceptable duration between an expected retry
	// being received and when it is being actually received. Manual local tests indicate
	// that this value is less than 20 ms.
	state.SetOrFail(ctx, t, retry_after.RetryAfterSecondsKey, 6)
	state.SetOrFail(ctx, t, retry_after.ExpectedIntervalMargingKey, 3)

	// Configure DataPlane & Send An Event
	env.Test(ctx, t, retry_after.ConfigureDataPlane(ctx, t))
	env.Test(ctx, t, retry_after.SendEvent(ctx, t))
}
