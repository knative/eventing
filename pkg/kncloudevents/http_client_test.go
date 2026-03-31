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

package kncloudevents

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/eventing/pkg/eventingtls"
)

var (
	testCaCerts string = `-----BEGIN CERTIFICATE-----
MIIDmjCCAoKgAwIBAgIUPpzLb+9RgdkYWxNb/G9ovq+geKcwDQYJKoZIhvcNAQEL
BQAwLzELMAkGA1UEBhMCVVMxIDAeBgNVBAMMF0tuYXRpdmUtRXhhbXBsZS1Sb290
LUNBMB4XDTI2MDMzMTE0MzI1NloXDTM2MDMyODE0MzI1NlowbTELMAkGA1UEBhMC
VVMxEjAQBgNVBAgMCVlvdXJTdGF0ZTERMA8GA1UEBwwIWW91ckNpdHkxHTAbBgNV
BAoMFEV4YW1wbGUtQ2VydGlmaWNhdGVzMRgwFgYDVQQDDA9sb2NhbGhvc3QubG9j
YWwwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDk9O0ajgZ4StkkGhE8
8auXKUvWbPUCuQsrRE2G9rxVsJOcay/R9z/FDN5KdeFUMXZwmllj7okaZVGwvMIg
aLbP0WIVQPBiMGx4+2zY6G/aYkx2CvzvVRN2gqjLogJNjPjIqp5/kpnfvy/lmklo
UHPzgSRCUskbZZeHnuNmoumTRFkNMvkwvSy+k/lbNx5a9JyoAtRX9mS8JriUdaTx
jtI+2VkXACoptbWwZXRT4eIfxvtpD9UhD9ebZuY9coySTleP+w/CWDQLUA60hrvK
WaBl4sXXQq24oQDpXciE6W7xatiDORJidQiYU3m6U3Ah1DkEPzHfgQn+7982yStB
Vm8PAgMBAAGjcDBuMB8GA1UdIwQYMBaAFF8Z48++4Hy+VlbLMShjG9nm3aCRMAkG
A1UdEwQCMAAwCwYDVR0PBAQDAgTwMBQGA1UdEQQNMAuCCWxvY2FsaG9zdDAdBgNV
HQ4EFgQUrmHARCOJiAJ7J/omW2YgsfYRPOowDQYJKoZIhvcNAQELBQADggEBAFd8
wfL1nqUeWMIszxx5QaJXH5HN1B9oDNV8vgkTf9csol41tR8cSUOpXJP97zWoexVv
xD9yZMZK0LFRfw9s2A8g5LV2eIq2gJ9zO0kzLi1z2N8UzElQnNmYJ6BRvUoBiLfM
GqKPmv4xAYzO1V7DcT/HSNDylipvrrlgDGi9sJZlb5pmB74+cGJplSixNOAULMTM
JmJKLL7sRaKeM2jTjbzyWFQCjTR0DQhQJwqtS8eFP2F+DiwgI9Rh56fcK5JvtAbU
+qehONc218Va3gyB34zPuuT5qL86NMBINoujvoKoEORc07QkPyqW+6baBPyedimL
oaaDMx38AzvCNWTG6v0=
-----END CERTIFICATE-----
`
)

func Test_getClientForAddressable(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		caCert  *string
		wantErr bool
	}{
		{
			name:    "Target with no CA certs",
			url:     "http://foo.bar",
			caCert:  nil,
			wantErr: false,
		},
		{
			name:    "Target with CA certs",
			url:     "https://foo.bar",
			caCert:  &testCaCerts,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := apis.ParseURL(tt.url)
			require.Nil(t, err)
			addressable := duckv1.Addressable{
				URL:     url,
				CACerts: tt.caCert,
			}
			_, err = getClientForAddressable(eventingtls.NewDefaultClientConfig(), addressable, otel.GetMeterProvider(), otel.GetTracerProvider())
			if (err != nil) != tt.wantErr {
				t.Errorf("getClientForAddressable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_ConfigureConnectionArgs(t *testing.T) {
	target := duckv1.Addressable{
		URL: apis.HTTP("foo.bar"),
	}

	// Set connection args
	ConfigureConnectionArgs(&ConnectionArgs{
		MaxIdleConnsPerHost: 1000,
		MaxIdleConns:        1000,
	})
	client1, err := getClientForAddressable(eventingtls.NewDefaultClientConfig(), target, otel.GetMeterProvider(), otel.GetTracerProvider())
	require.Nil(t, err)

	// Set other connection args
	ConfigureConnectionArgs(&ConnectionArgs{
		MaxIdleConnsPerHost: 2000,
		MaxIdleConns:        2000,
	})
	client2, err := getClientForAddressable(eventingtls.NewDefaultClientConfig(), target, otel.GetMeterProvider(), otel.GetTracerProvider())
	require.Nil(t, err)

	// Try to set the same value and client should not be cleaned up
	ConfigureConnectionArgs(&ConnectionArgs{
		MaxIdleConnsPerHost: 2000,
		MaxIdleConns:        2000,
	})
	client2_2, err := getClientForAddressable(eventingtls.NewDefaultClientConfig(), target, otel.GetMeterProvider(), otel.GetTracerProvider())
	require.Nil(t, err)
	require.Same(t, client2_2, client2)

	// Set back to nil
	ConfigureConnectionArgs(nil)
	client3, err := getClientForAddressable(eventingtls.NewDefaultClientConfig(), target, otel.GetMeterProvider(), otel.GetTracerProvider())
	require.Nil(t, err)

	require.NotSame(t, client1, client2)
	require.NotSame(t, client1, client3)
	require.NotSame(t, client2, client3)
}
