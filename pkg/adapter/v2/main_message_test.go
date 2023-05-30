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

package adapter

import (
	"context"
	"os"
	"testing"

	"go.opencensus.io/stats/view"
	"knative.dev/eventing/pkg/eventingtls/eventingtlstesting"
	"knative.dev/eventing/pkg/metrics/source"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/leaderelection"
	"knative.dev/pkg/metrics"

	_ "knative.dev/pkg/client/injection/kube/client/fake"
)

type myAdapterBindings struct{}

func TestMainMessageAdapterWithContext(t *testing.T) {
	m := &metrics.ExporterOptions{
		Domain:         "example.com",
		Component:      "foo",
		PrometheusPort: 9021,
		PrometheusHost: "prom.example.com",
		ConfigMap: map[string]string{
			"profiling.enable": "true",
			"foo":              "bar",
		},
	}
	metricsJson, _ := metrics.OptionsToJSON(m)

	os.Setenv("K_SINK", "http://sink")
	os.Setenv("K_CA_CERTS", string(eventingtlstesting.CA))
	os.Setenv("NAMESPACE", "ns")
	os.Setenv("K_METRICS_CONFIG", metricsJson)
	os.Setenv("K_LOGGING_CONFIG", "logging")
	os.Setenv("MODE", "mymode")

	defer func() {
		os.Unsetenv("K_SINK")
		os.Unsetenv("K_CA_CERTS")
		os.Unsetenv("NAMESPACE")
		os.Unsetenv("K_METRICS_CONFIG")
		os.Unsetenv("K_LOGGING_CONFIG")
		os.Unsetenv("MODE")
	}()

	ctx, cancel := context.WithCancel(context.TODO())

	MainMessageAdapterWithContext(ctx,
		"mycomponentbindings",
		func() EnvConfigAccessor { return &myEnvConfig{} },
		func(ctx context.Context, environment EnvConfigAccessor, sink duckv1.Addressable, reporter source.StatsReporter) MessageAdapter {
			env := environment.(*myEnvConfig)
			if env.Mode != "mymode" {
				t.Error("Expected mode mymode, got:", env.Mode)
			}

			if sink.URL.String() != "http://sink" {
				t.Error("Expected sink.URL http://sink, got:", sink.URL.String())
			}

			if sink.CACerts == nil || *sink.CACerts != string(eventingtlstesting.CA) {
				t.Error("Unexpected sink.CACerts, got", sink.CACerts)
			}

			if leaderelection.HasLeaderElection(ctx) {
				t.Error("Expected no leader election, but got leader election")
			}
			return &myAdapterBindings{}
		})

	cancel()

	defer view.Unregister(metrics.NewMemStatsAll().DefaultViews()...)
}

func (m *myAdapterBindings) Start(_ context.Context) error {
	return nil
}
