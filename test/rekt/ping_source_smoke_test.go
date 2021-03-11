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
	"knative.dev/eventing/test/rekt/resources/svc"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/feature"
	"strconv"
	"testing"

	_ "knative.dev/pkg/system/testing"

	"knative.dev/eventing/test/rekt/features/pingsource"
	ps "knative.dev/eventing/test/rekt/resources/pingsource"
)

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

	configs := [][]ps.CfgFn{
		{},
		{ps.WithData("application/json", `{"hello":"world"}`)},
		{ps.WithData("text/plain", "hello, world!")},
		{ps.WithDataBase64("application/json", "eyJoZWxsbyI6IndvcmxkIn0=")},
		{ps.WithDataBase64("text/plain", "aGVsbG8sIHdvcmxkIQ==")},
	}

	for _, n := range names {
		for i, cfg := range configs {
			name := n + strconv.Itoa(i) // Make the name unique for each config.
			if len(name) >= 64 {
				name = name[:63] // 63 is the max length.
			}

			f := pingsource.GoesReady(name)

			sink := feature.MakeRandomK8sName("sink")
			f.Setup("install a Service", svc.Install(sink, "app", "rekt"))

			cfg = append(cfg, ps.WithSink(&duckv1.KReference{
				Kind:       "Service",
				Name:       sink,
				APIVersion: "v1",
			}, ""))

			f.Setup("install a PingSource", ps.Install(name, cfg...))

			env.Test(ctx, t, f)
		}
	}
}
