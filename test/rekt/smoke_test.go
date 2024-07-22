//go:build e2e
// +build e2e

/*
Copyright 2020 The Knative Authors

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
	"strconv"
	"testing"

	"k8s.io/utils/pointer"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/test/rekt/features/apiserversource"
	"knative.dev/eventing/test/rekt/features/broker"
	"knative.dev/eventing/test/rekt/features/containersource"
	"knative.dev/eventing/test/rekt/features/parallel"
	"knative.dev/eventing/test/rekt/features/pingsource"
	"knative.dev/eventing/test/rekt/features/sequence"
	"knative.dev/eventing/test/rekt/features/sinkbinding"
	b "knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/eventing/test/rekt/resources/channel_template"
	"knative.dev/eventing/test/rekt/resources/delivery"
	presources "knative.dev/eventing/test/rekt/resources/parallel"
	ps "knative.dev/eventing/test/rekt/resources/pingsource"
	sresources "knative.dev/eventing/test/rekt/resources/sequence"
	sb "knative.dev/eventing/test/rekt/resources/sinkbinding"
	_ "knative.dev/pkg/system/testing"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	"knative.dev/reconciler-test/pkg/resources/deployment"
	"knative.dev/reconciler-test/pkg/resources/service"
)

const (
	heartbeatsImage = "ko://knative.dev/eventing/cmd/heartbeats"
)

// TestSmoke_Broker
func TestSmoke_Broker(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment()
	t.Cleanup(env.Finish)

	names := []string{
		"default",
		"customname",
		"name-with-dash",
		"name1with2numbers3",
		"name63-01234567890123456789012345678901234567890123456789012345",
	}

	for _, name := range names {
		env.Test(ctx, t, broker.GoesReady(name, b.WithEnvConfig()...))
	}
}

// TestSmoke_Trigger
func TestSmoke_Trigger(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment()
	t.Cleanup(env.Finish)

	names := []string{
		"default",
		"customname",
		"name-with-dash",
		"name1with2numbers3",
		"name63-01234567890123456789012345678901234567890123456789012345",
	}
	brokerName := "broker-rekt"

	env.Prerequisite(ctx, t, broker.GoesReady(brokerName, b.WithEnvConfig()...))

	for _, name := range names {
		env.Test(ctx, t, broker.TriggerGoesReady(name, brokerName))
	}
}

// TestSmoke_PingSource
func TestSmoke_PingSource(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment()
	t.Cleanup(env.Finish)

	names := []string{
		"customname",
		"name-with-dash",
		"name1with2numbers3",
		"name63-0123456789012345678901234567890123456789012345678901234",
	}

	configs := [][]manifest.CfgFn{
		{},
		{ps.WithData("application/json", `{"hello":"world"}`)},
		{ps.WithData("text/plain", "hello, world!")},
		{ps.WithDataBase64("application/json", "eyJoZWxsbyI6IndvcmxkIn0=")},
		{ps.WithDataBase64("text/plain", "aGVsbG8sIHdvcmxkIQ==")},
	}

	for _, name := range names {
		for i, cfg := range configs {
			n := name + strconv.Itoa(i) // Make the name unique for each config.
			if len(n) >= 64 {
				n = n[:63] // 63 is the max length.
			}
			env.Test(ctx, t, pingsource.PingSourceGoesReady(n, cfg...))
		}
	}
}

// TestSmoke_ContainerSource
func TestSmoke_ContainerSource(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment()
	t.Cleanup(env.Finish)

	names := []string{
		"customname",
		"name-with-dash",
		"name1with2numbers3",
		"name63-0123456789012345678901234567890123456789012345678901234",
	}

	for _, name := range names {
		env.Test(ctx, t, containersource.GoesReady(name))
	}
}

// TestSmoke_ApiServerSource
func TestSmoke_ApiServerSource(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment()
	t.Cleanup(env.Finish)

	names := []string{
		"customname",
		"name-with-dash",
		"name1with2numbers3",
		"name63-0123456789012345678901234567890123456789012345678901234",
	}

	for _, name := range names {
		env.Test(ctx, t, apiserversource.Install(name))
		env.Test(ctx, t, apiserversource.GoesReady(name))
	}
}

// TestSmoke_Parallel
func TestSmoke_Parallel(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment()
	t.Cleanup(env.Finish)

	names := []string{
		"customname",
		"name-with-dash",
		"name1with2numbers3",
		"name63-0123456789012345678901234567890123456789012345678901234",
	}

	for _, name := range names {
		env.Test(ctx, t, parallel.GoesReady(name))
	}
}

// TestSmoke_ParallelDelivery
func TestSmoke_ParallelDelivery(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment()
	t.Cleanup(env.Finish)

	names := []string{
		"customname",
		"name-with-dash",
		"name1with2numbers3",
		"name63-0123456789012345678901234567890123456789012345678901234",
	}

	for _, name := range names {
		template := channel_template.ChannelTemplate{
			TypeMeta: channel_impl.TypeMeta(),
			Spec:     map[string]interface{}{},
		}
		SpecDelivery(template.Spec)
		env.Test(ctx, t, parallel.GoesReady(name, presources.WithChannelTemplate(template)))
	}
}

// TestSmoke_Parallel
func TestSmoke_Parallel_with_no_filter(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment()
	t.Cleanup(env.Finish)

	names := []string{
		"customname",
		"name-with-dash",
		"name1with2numbers3",
		"name63-0123456789012345678901234567890123456789012345678901234",
	}

	for _, name := range names {
		env.Test(ctx, t, parallel.GoesReadyWithoutFilters(name))
	}
}

// TestSmoke_Sequence
func TestSmoke_Sequence(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment()
	t.Cleanup(env.Finish)

	names := []string{
		"customname",
		"name-with-dash",
		"name1with2numbers3",
		"name63-0123456789012345678901234567890123456789012345678901234",
	}

	for _, name := range names {
		template := channel_template.ChannelTemplate{
			TypeMeta: channel_impl.TypeMeta(),
			Spec:     map[string]interface{}{},
		}
		env.Test(ctx, t, sequence.GoesReady(name, sresources.WithChannelTemplate(template)))
	}
}

// TestSmoke_SequenceDelivery
func TestSmoke_SequenceDelivery(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment()
	t.Cleanup(env.Finish)

	names := []string{
		"customname",
		"name-with-dash",
		"name1with2numbers3",
		"name63-0123456789012345678901234567890123456789012345678901234",
	}

	for _, name := range names {
		template := channel_template.ChannelTemplate{
			TypeMeta: channel_impl.TypeMeta(),
			Spec:     map[string]interface{}{},
		}
		SpecDelivery(template.Spec)
		env.Test(ctx, t, sequence.GoesReady(name, sresources.WithChannelTemplate(template)))
	}
}

func SpecDelivery(spec map[string]interface{}) {
	linear := eventingduck.BackoffPolicyLinear
	delivery.WithRetry(10, &linear, pointer.String("PT1S"))(spec)
}

// TestSmoke_SinkBinding
func TestSmoke_SinkBinding(t *testing.T) {
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
		f := sinkbinding.GoesReady(name)

		sink := feature.MakeRandomK8sName("sink")
		f.Setup("install a service", service.Install(sink,
			service.WithSelectors(map[string]string{"app": "rekt"})))

		subject := feature.MakeRandomK8sName("subject")
		f.Setup("install a deployment", deployment.Install(subject, heartbeatsImage,
			deployment.WithEnvs(map[string]string{
				"POD_NAME":      "heartbeats",
				"POD_NAMESPACE": env.Namespace(),
			})))

		f.Setup("install a sinkbinding", sb.Install(name, service.AsDestinationRef(sink), deployment.AsTrackerReference(subject)))

		env.Test(ctx, t, f)
	}
}
