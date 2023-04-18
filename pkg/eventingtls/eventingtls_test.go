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

package eventingtls

import (
	"crypto/tls"
	"crypto/x509"
	"testing"

	"k8s.io/utils/pointer"
)

func TestGetClientConfig(t *testing.T) {
	t.Parallel()

	sysCertPool, err := x509.SystemCertPool()
	if err != nil {
		t.Fatal(err)
	}
	sysCertPool = sysCertPool.Clone()

	pemCaCert := `
-----BEGIN CERTIFICATE-----
MIIDLTCCAhWgAwIBAgIJAOjtl0zhGBvpMA0GCSqGSIb3DQEBCwUAMC0xEzARBgNV
BAoMCmlvLnN0cmltemkxFjAUBgNVBAMMDWNsdXN0ZXItY2EgdjAwHhcNMjAxMjIw
MDgzNzU4WhcNMjExMjIwMDgzNzU4WjAtMRMwEQYDVQQKDAppby5zdHJpbXppMRYw
FAYDVQQDDA1jbHVzdGVyLWNhIHYwMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB
CgKCAQEAy7rIo+UwJh5dL6PhUfDe9wRuLgOf1ZeZmabd++eLc2kWL1r6TO8X034n
CerkREfjF+MjDoK30z9xvEURThoSi20a4i/Cb39on9T0AgOr5qCSrqlN9n4KtRey
ZLnKKA5QyLAM6kzyyvIg4PVwWCWFTQSicDPzqd2OmH6jtogD50FkbaP7LcyrKnWf
64gcR9CCEAcrO8tJdhcZP2Slxg+RvupVjXK1rdZcI6/liZ3Jp4hzApSRN30x/8wU
5eJYAtzaeWUvJ0Yq/7BH7uY8J+2Hwh+shhi5K98HBAKeISwuIJEQrWmmUer8WGp1
IcBZqXbkd4dBXuFa0chO0gSKvzjKpQIDAQABo1AwTjAdBgNVHQ4EFgQUeascji1L
C2voPwDAlPL6iz8TzncwHwYDVR0jBBgwFoAUeascji1LC2voPwDAlPL6iz8Tzncw
DAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAIEL2uustCrpPas06LyoR
VR6QFHQJDcMgdL0CZFE46uLSgGupXO0ybmPP2ymBJ1zDNxx1qskNTwBsfBJLBAj6
8LfJmhfw98QK8YQDJ/Xhx3fcVxjn6NjJ3RYOyb5bqSIGGCQZRmbMjerf71KMhP3X
rdYg2hVoCvfRcfP2G0jbWtMRK4+MlB3oEvhIvQQW1dw4sohw32HaNJnzb7dErEDB
Ha2zVM47CcNezdWYUD5NQzFqCRypgrIONafQI2S+Ck7aKOiqF03QSug4wizRbKhT
uYpQg59dUIOBebg0roRF326H2x6kFGn5L2o+TROrZeeXT8vyIl2R33o3E+ULpuw+
Vw==
-----END CERTIFICATE-----
`

	tt := []struct {
		name     string
		cfg      ClientConfig
		expected tls.Config
		wantErr  bool
	}{
		{
			name: "empty string CA certs",
			cfg: ClientConfig{
				CACerts: pointer.String(""),
			},
			expected: tls.Config{
				MinVersion: tls.VersionTLS12,
				RootCAs:    sysCertPool,
			},
		},
		{
			name: "nil CA certs",
			cfg:  ClientConfig{},
			expected: tls.Config{
				MinVersion: tls.VersionTLS12,
				RootCAs:    sysCertPool,
			},
		},
		{
			name: "Additional CA certs",
			cfg: ClientConfig{
				CACerts: pointer.String(pemCaCert),
			},
			expected: tls.Config{
				MinVersion: tls.VersionTLS12,
				RootCAs:    WithCerts(sysCertPool, pemCaCert),
			},
		},
		{
			name: "Additional broken CA certs",
			cfg: ClientConfig{
				CACerts: pointer.String(pemCaCert[:len(pemCaCert)-30]),
			},
			expected: tls.Config{
				MinVersion: tls.VersionTLS12,
			},
			wantErr: true,
		},
	}

	for i := range tt {
		tc := &tt[i]
		t.Run(tc.name, func(t *testing.T) {
			got, err := GetTLSClientConfig(tc.cfg)
			if tc.wantErr != (err != nil) {
				t.Fatalf("Want err: %v, got %v", tc.wantErr, err)
			}
			if err != nil {
				return
			}

			if !got.RootCAs.Equal(tc.expected.RootCAs) {
				t.Fatalf("Got RootCAs are not equal to expected RootCAs")
			}

			if got.MinVersion != tc.expected.MinVersion {
				t.Fatalf("want MinVersion %v, got %v", tc.expected.MinVersion, got.MinVersion)
			}
		})
	}
}

func WithCerts(pool *x509.CertPool, caCerts string) *x509.CertPool {
	pool = pool.Clone()
	if ok := pool.AppendCertsFromPEM([]byte(caCerts)); !ok {
		panic("Failed to append CA certs from PEM:\n" + caCerts)
	}
	return pool
}
