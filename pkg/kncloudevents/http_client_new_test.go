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
	nethttp "net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opencensus.io/plugin/ochttp"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	testCaCerts string = `-----BEGIN CERTIFICATE-----
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

func Test_getClientForAddressable(t *testing.T) {
	tests := []struct {
		name                string
		url                 string
		caCert              *string
		wantTLSRootCAConfig bool
		wantErr             bool
	}{
		{
			name:                "Target with no CA certs",
			url:                 "http://foo.bar",
			caCert:              nil,
			wantTLSRootCAConfig: false,
			wantErr:             false,
		},
		{
			name:                "Target with CA certs",
			url:                 "https://foo.bar",
			caCert:              &testCaCerts,
			wantTLSRootCAConfig: true,
			wantErr:             false,
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
			got, err := getClientForAddressable(addressable)
			if (err != nil) != tt.wantErr {
				t.Errorf("getClientForAddressable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			clientTransport := castToTransport(got)
			if tt.wantTLSRootCAConfig != (clientTransport.TLSClientConfig.RootCAs != nil) {
				t.Errorf("wantTLSRootCAConfig = %v, but client has TLS client RootCAs config = %v", tt.wantTLSRootCAConfig, clientTransport.TLSClientConfig.RootCAs != nil)
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
	client1, err := getClientForAddressable(target)
	require.Nil(t, err)

	require.Equal(t, 1000, castToTransport(client1).MaxIdleConns)
	require.Equal(t, 1000, castToTransport(client1).MaxIdleConnsPerHost)

	// Set other connection args
	ConfigureConnectionArgs(&ConnectionArgs{
		MaxIdleConnsPerHost: 2000,
		MaxIdleConns:        2000,
	})
	client2, err := getClientForAddressable(target)
	require.Nil(t, err)

	require.Equal(t, 2000, castToTransport(client2).MaxIdleConns)
	require.Equal(t, 2000, castToTransport(client2).MaxIdleConnsPerHost)

	// Try to set the same value and client should not be cleaned up
	ConfigureConnectionArgs(&ConnectionArgs{
		MaxIdleConnsPerHost: 2000,
		MaxIdleConns:        2000,
	})
	client2_2, err := getClientForAddressable(target)
	require.Nil(t, err)
	require.Same(t, client2_2, client2)

	// Set back to nil
	ConfigureConnectionArgs(nil)
	client3, err := getClientForAddressable(target)
	require.Nil(t, err)

	require.Equal(t, nethttp.DefaultTransport.(*nethttp.Transport).MaxIdleConns, castToTransport(client3).MaxIdleConns)
	require.Equal(t, nethttp.DefaultTransport.(*nethttp.Transport).MaxIdleConnsPerHost, castToTransport(client3).MaxIdleConnsPerHost)

	require.NotSame(t, client1, client2)
	require.NotSame(t, client1, client3)
	require.NotSame(t, client2, client3)
}

func castToTransport(client *nethttp.Client) *nethttp.Transport {
	return client.Transport.(*ochttp.Transport).Base.(*nethttp.Transport)
}
