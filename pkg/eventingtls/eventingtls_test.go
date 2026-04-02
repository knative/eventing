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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"k8s.io/utils/pointer"
	pkgtls "knative.dev/pkg/network/tls"
)

func TestGetClientConfig(t *testing.T) {
	t.Parallel()

	sysCertPool, err := x509.SystemCertPool()
	if err != nil {
		t.Fatal(err)
	}
	sysCertPool = sysCertPool.Clone()
	pemCaCert := generateTestCACertPEM()

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

func generateTestCACertPEM() string {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic("failed to generate CA key: " + err.Error())
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Country:    []string{"US"},
			CommonName: "Knative-Example-Root-CA",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		panic("failed to create CA certificate: " + err.Error())
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func WithCerts(pool *x509.CertPool, caCerts string) *x509.CertPool {
	pool = pool.Clone()
	if ok := pool.AppendCertsFromPEM([]byte(caCerts)); !ok {
		panic("Failed to append CA certs from PEM:\n" + caCerts)
	}
	return pool
}

func TestGetTLSClientConfigEnv(t *testing.T) {
	t.Run("defaults to TLS 1.2 when env not set", func(t *testing.T) {
		t.Setenv(pkgtls.MinVersionEnvKey, "")

		cfg, err := GetTLSClientConfig(NewDefaultClientConfig())
		if err != nil {
			t.Fatal("unexpected error:", err)
		}
		if cfg.MinVersion != tls.VersionTLS12 {
			t.Fatalf("want MinVersion TLS 1.2 (%d), got %d", tls.VersionTLS12, cfg.MinVersion)
		}
	})

	t.Run("uses TLS 1.3 when explicitly set via env", func(t *testing.T) {
		t.Setenv(pkgtls.MinVersionEnvKey, "1.3")

		cfg, err := GetTLSClientConfig(NewDefaultClientConfig())
		if err != nil {
			t.Fatal("unexpected error:", err)
		}
		if cfg.MinVersion != tls.VersionTLS13 {
			t.Fatalf("want MinVersion TLS 1.3 (%d), got %d", tls.VersionTLS13, cfg.MinVersion)
		}
	})

	t.Run("reads MaxVersion from env", func(t *testing.T) {
		t.Setenv(pkgtls.MaxVersionEnvKey, "1.3")

		cfg, err := GetTLSClientConfig(NewDefaultClientConfig())
		if err != nil {
			t.Fatal("unexpected error:", err)
		}
		if cfg.MaxVersion != tls.VersionTLS13 {
			t.Fatalf("want MaxVersion TLS 1.3 (%d), got %d", tls.VersionTLS13, cfg.MaxVersion)
		}
	})

	t.Run("reads CipherSuites from env", func(t *testing.T) {
		t.Setenv(pkgtls.CipherSuitesEnvKey, "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256")

		cfg, err := GetTLSClientConfig(NewDefaultClientConfig())
		if err != nil {
			t.Fatal("unexpected error:", err)
		}
		if len(cfg.CipherSuites) != 1 || cfg.CipherSuites[0] != tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 {
			t.Fatalf("want CipherSuites [%d], got %v", tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, cfg.CipherSuites)
		}
	})

	t.Run("reads CurvePreferences from env", func(t *testing.T) {
		t.Setenv(pkgtls.CurvePreferencesEnvKey, "X25519,CurveP256")

		cfg, err := GetTLSClientConfig(NewDefaultClientConfig())
		if err != nil {
			t.Fatal("unexpected error:", err)
		}
		if len(cfg.CurvePreferences) != 2 {
			t.Fatalf("want 2 CurvePreferences, got %d", len(cfg.CurvePreferences))
		}
		if cfg.CurvePreferences[0] != tls.X25519 || cfg.CurvePreferences[1] != tls.CurveP256 {
			t.Fatalf("want CurvePreferences [X25519, CurveP256], got %v", cfg.CurvePreferences)
		}
	})

	t.Run("returns error on invalid env value", func(t *testing.T) {
		t.Setenv(pkgtls.MinVersionEnvKey, "invalid")

		_, err := GetTLSClientConfig(NewDefaultClientConfig())
		if err == nil {
			t.Fatal("expected error for invalid TLS_MIN_VERSION, got nil")
		}
	})
}

func TestGetTLSServerConfig(t *testing.T) {
	t.Run("defaults to TLS 1.2 when env not set", func(t *testing.T) {
		t.Setenv(pkgtls.MinVersionEnvKey, "")

		cfg, err := GetTLSServerConfig(NewDefaultServerConfig())
		if err != nil {
			t.Fatal("unexpected error:", err)
		}
		if cfg.MinVersion != tls.VersionTLS12 {
			t.Fatalf("want MinVersion TLS 1.2 (%d), got %d", tls.VersionTLS12, cfg.MinVersion)
		}
	})

	t.Run("uses TLS 1.3 when explicitly set via env", func(t *testing.T) {
		t.Setenv(pkgtls.MinVersionEnvKey, "1.3")

		cfg, err := GetTLSServerConfig(NewDefaultServerConfig())
		if err != nil {
			t.Fatal("unexpected error:", err)
		}
		if cfg.MinVersion != tls.VersionTLS13 {
			t.Fatalf("want MinVersion TLS 1.3 (%d), got %d", tls.VersionTLS13, cfg.MinVersion)
		}
	})

	t.Run("uses TLS 1.2 when explicitly set via env", func(t *testing.T) {
		t.Setenv(pkgtls.MinVersionEnvKey, "1.2")

		cfg, err := GetTLSServerConfig(NewDefaultServerConfig())
		if err != nil {
			t.Fatal("unexpected error:", err)
		}
		if cfg.MinVersion != tls.VersionTLS12 {
			t.Fatalf("want MinVersion TLS 1.2 (%d), got %d", tls.VersionTLS12, cfg.MinVersion)
		}
	})

	t.Run("reads MaxVersion from env", func(t *testing.T) {
		t.Setenv(pkgtls.MaxVersionEnvKey, "1.3")

		cfg, err := GetTLSServerConfig(NewDefaultServerConfig())
		if err != nil {
			t.Fatal("unexpected error:", err)
		}
		if cfg.MaxVersion != tls.VersionTLS13 {
			t.Fatalf("want MaxVersion TLS 1.3 (%d), got %d", tls.VersionTLS13, cfg.MaxVersion)
		}
	})

	t.Run("reads CipherSuites from env", func(t *testing.T) {
		t.Setenv(pkgtls.CipherSuitesEnvKey, "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256")

		cfg, err := GetTLSServerConfig(NewDefaultServerConfig())
		if err != nil {
			t.Fatal("unexpected error:", err)
		}
		if len(cfg.CipherSuites) != 1 || cfg.CipherSuites[0] != tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 {
			t.Fatalf("want CipherSuites [%d], got %v", tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, cfg.CipherSuites)
		}
	})

	t.Run("reads CurvePreferences from env", func(t *testing.T) {
		t.Setenv(pkgtls.CurvePreferencesEnvKey, "X25519,CurveP256")

		cfg, err := GetTLSServerConfig(NewDefaultServerConfig())
		if err != nil {
			t.Fatal("unexpected error:", err)
		}
		if len(cfg.CurvePreferences) != 2 {
			t.Fatalf("want 2 CurvePreferences, got %d", len(cfg.CurvePreferences))
		}
		if cfg.CurvePreferences[0] != tls.X25519 || cfg.CurvePreferences[1] != tls.CurveP256 {
			t.Fatalf("want CurvePreferences [X25519, CurveP256], got %v", cfg.CurvePreferences)
		}
	})

	t.Run("returns error on invalid env value", func(t *testing.T) {
		t.Setenv(pkgtls.MinVersionEnvKey, "invalid")

		_, err := GetTLSServerConfig(NewDefaultServerConfig())
		if err == nil {
			t.Fatal("expected error for invalid TLS_MIN_VERSION, got nil")
		}
	})

	t.Run("preserves GetCertificate callback", func(t *testing.T) {
		called := false
		sc := ServerConfig{
			GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				called = true
				return nil, nil
			},
		}
		cfg, err := GetTLSServerConfig(sc)
		if err != nil {
			t.Fatal("unexpected error:", err)
		}
		if cfg.GetCertificate == nil {
			t.Fatal("GetCertificate should not be nil")
		}
		_, _ = cfg.GetCertificate(nil)
		if !called {
			t.Fatal("GetCertificate callback was not invoked")
		}
	})
}
