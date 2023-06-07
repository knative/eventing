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
MIIDPzCCAiegAwIBAgIUOF3U5UMwffSmdo24IVU1k+qix3YwDQYJKoZIhvcNAQEL
BQAwLzELMAkGA1UEBhMCVVMxIDAeBgNVBAMMF0tuYXRpdmUtRXhhbXBsZS1Sb290
LUNBMB4XDTIzMDYwNjE0MDY1NFoXDTI2MDMyNjE0MDY1NFowLzELMAkGA1UEBhMC
VVMxIDAeBgNVBAMMF0tuYXRpdmUtRXhhbXBsZS1Sb290LUNBMIIBIjANBgkqhkiG
9w0BAQEFAAOCAQ8AMIIBCgKCAQEAsJEA/+FW8e/ChmpseeH+UMtpP3PIq4VO26yh
fg3RSWKRbEnpkusWX6tM5NIZ9HqZOhB9dvb0OAC+YBM5ce8eA1/5tIUcxOzvMo5S
Oe+5cOgzZPLNesPBD+vteFXeD/9Hg75KfxctgyYfKqAE4Q8afaxs29/9K4wZkdE7
Fs4ED8r6hxf+7wgVSurnHiQnupHOb3BCQEGFm4w5/YJMhJFM29+LtIa5iZvQdlIC
zrIiLSckaRCiuJH2U5HCxk6WpodyoD5ffqzX7/+xismUwsX9opnMfdz7vT4ZYvKc
5O0u6/mx9fvhCL7hVwz8/FKvd1+Z4WnGoL/Iz3g+T/qdMbA+1wIDAQABo1MwUTAd
BgNVHQ4EFgQU51Q84l/eECxUhLRPhlcoLougg0owHwYDVR0jBBgwFoAU51Q84l/e
ECxUhLRPhlcoLougg0owDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOC
AQEAZVXtix62c6VVAEZHsSTPwlMwGjZ67UCd6NxeY5IgXdT/vmorlrsoZa0FYYkU
TdWOHt7Q1C48W+tA2yMTPGs240Zradam2CXAxEvL7/aC6GEFs7vhkq6riwJ/erR3
ZAZjcWi5Qk03q7eS61JJvaV9+fKg+F2BB2EqaCPo7HMMSXO81aeHEMl/AQsNPnur
2VG1tchMQvfakRf53H1hWu5h4APuZo1MTkPmBOTLZG7eAJTtfVWz1aPwB1rUMCyP
wSdZWoEx7ye2kUHEyRKdRGbHyJtY9YYvaROznzxqVpIqHxnRQnE/If7kcN4t/7vi
28zWIDKzJ8je40SPcLSfplRvBQ==
-----END CERTIFICATE-----`

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
