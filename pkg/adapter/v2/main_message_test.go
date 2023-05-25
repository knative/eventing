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
	os.Setenv("K_CA_CERTS", testCaCerts)
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

			if sink.CACerts == nil || *sink.CACerts != testCaCerts {
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

const (
	testCaCerts = `-----BEGIN CERTIFICATE-----
MIIDmjCCAoKgAwIBAgIUYzA4bTMXevuk3pl2Mn8hpCYL2C0wDQYJKoZIhvcNAQEL
BQAwLzELMAkGA1UEBhMCVVMxIDAeBgNVBAMMF0tuYXRpdmUtRXhhbXBsZS1Sb290
LUNBMB4XDTIzMDQwNTEzMTUyNFoXDTI2MDEyMzEzMTUyNFowbTELMAkGA1UEBhMC
VVMxEjAQBgNVBAgMCVlvdXJTdGF0ZTERMA8GA1UEBwwIWW91ckNpdHkxHTAbBgNV
BAoMFEV4YW1wbGUtQ2VydGlmaWNhdGVzMRgwFgYDVQQDDA9sb2NhbGhvc3QubG9j
YWwwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC5teo+En6U5nhqn7Sc
uanqswUmPlgs9j/8l21Rhb4T+ezlYKGQGhbJyFFMuiCE1Rjn8bpCwi7Nnv12Y2nz
FhEv2Jx0yL3Tqx0Q593myqKDq7326EtbO7wmDT0XD03twH5i9XZ0L0ihPWn1mjUy
WxhnHhoFpXrsnQECJorZY6aTrFbGVYelIaj5AriwiqyL0fET8pueI2GwLjgWHFSH
X8XsGAlcLUhkQG0Z+VO9usy4M1Wpt+cL6cnTiQ+sRmZ6uvaj8fKOT1Slk/oUeAi4
WqFkChGzGzLik0QrhKGTdw3uUvI1F2sdQj0GYzXaWqRz+tP9qnXdzk1GrszKKSlm
WBTLAgMBAAGjcDBuMB8GA1UdIwQYMBaAFJJcCftus4vj98N0zQQautsjEu82MAkG
A1UdEwQCMAAwCwYDVR0PBAQDAgTwMBQGA1UdEQQNMAuCCWxvY2FsaG9zdDAdBgNV
HQ4EFgQUnu/3vqA3VEzm128x/hLyZzR9JlgwDQYJKoZIhvcNAQELBQADggEBAFc+
1cKt/CNjHXUsirgEhry2Mm96R6Yxuq//mP2+SEjdab+FaXPZkjHx118u3PPX5uTh
gTT7rMfka6J5xzzQNqJbRMgNpdEFH1bbc11aYuhi0khOAe0cpQDtktyuDJQMMv3/
3wu6rLr6fmENo0gdcyUY9EiYrglWGtdXhlo4ySRY8UZkUScG2upvyOhHTxVCAjhP
efbMkNjmDuZOMK+wqanqr5YV6zMPzkQK7DspfRgasMAQmugQu7r2MZpXg8Ilhro1
s/wImGnMVk5RzpBVrq2VB9SkX/ThTVYEC/Sd9BQM364MCR+TA1l8/ptaLFLuwyw8
O2dgzikq8iSy1BlRsVw=
-----END CERTIFICATE-----
`
)
