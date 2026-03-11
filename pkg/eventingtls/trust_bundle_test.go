/*
Copyright 2025 The Knative Authors

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
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// parseCertsFromPEM parses all certificates from PEM-encoded data
func parseCertsFromPEM(t *testing.T, pemData []byte) []*x509.Certificate {
	var certs []*x509.Certificate
	remaining := pemData
	for len(remaining) > 0 {
		var block *pem.Block
		block, remaining = pem.Decode(remaining)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatalf("Failed to parse certificate: %v", err)
		}
		certs = append(certs, cert)
	}
	return certs
}

func TestCombineValidTrustBundles(t *testing.T) {
	// Helper function to create a valid test certificate
	generateValidCert := func() []byte {
		template := x509.Certificate{
			SerialNumber: big.NewInt(1),
			Subject: pkix.Name{
				CommonName: "test.example.com",
			},
			NotBefore:             time.Now(),
			NotAfter:              time.Now().Add(time.Hour * 24),
			KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			BasicConstraintsValid: true,
			IsCA:                  true,
		}

		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("failed to generate private key: %s", err)
		}
		derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
		if err != nil {
			t.Fatalf("Failed to create certificate: %v", err)
		}

		var certPEM bytes.Buffer
		if err := pem.Encode(&certPEM, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
			t.Fatalf("Failed to encode certificate: %v", err)
		}
		return certPEM.Bytes()
	}

	// Generate invalid PEM data (not a certificate)
	generateInvalidPEMData := func() []byte {
		// Generate an RSA private key (which is not a certificate)
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("Failed to generate private key: %v", err)
		}

		// Encode it to PEM format
		privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
		pemBlock := &pem.Block{
			Type:  "RSA PRIVATE KEY", // Not a CERTIFICATE type
			Bytes: privateKeyBytes,
		}

		return pem.EncodeToMemory(pemBlock)
	}

	// Generate some test certificates
	validCert1 := generateValidCert()
	validCert2 := generateValidCert()
	validCert3 := generateValidCert()

	// Invalid PEM data (not a certificate)
	invalidTypeData := generateInvalidPEMData()

	// Invalid certificate data (corrupted)
	invalidCertData := []byte(`-----BEGIN CERTIFICATE-----
MIIDIzCC corrupted certificate data
-----END CERTIFICATE-----`)

	// Empty PEM data
	emptyData := []byte("")

	tests := []struct {
		name        string
		configMaps  []*corev1.ConfigMap
		expected    string
		expectError bool
	}{
		{
			name: "Single ConfigMap with one valid certificate",
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cm1",
						Namespace: "test",
					},
					Data: map[string]string{
						"cert.pem": string(validCert1),
					},
				},
			},
			expected:    string(validCert1),
			expectError: false,
		},
		{
			name: "Multiple ConfigMaps with valid certificates",
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cm1",
						Namespace: "test",
					},
					Data: map[string]string{
						"cert1.pem": string(validCert1),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cm2",
						Namespace: "test",
					},
					Data: map[string]string{
						"cert2.pem": string(validCert2),
					},
				},
			},
			expected:    string(validCert1) + string(validCert2),
			expectError: false,
		},
		{
			name: "Multiple ConfigMaps in reversed order produces consistent output",
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cm2",
						Namespace: "test",
					},
					Data: map[string]string{
						"cert2.pem": string(validCert2),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cm1",
						Namespace: "test",
					},
					Data: map[string]string{
						"cert1.pem": string(validCert1),
					},
				},
			},
			expected:    string(validCert1) + string(validCert2),
			expectError: false,
		},
		{
			name: "Multiple certificates in same key",
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cm1",
						Namespace: "test",
					},
					Data: map[string]string{
						"certs.pem": string(validCert1) + string(validCert2),
					},
				},
			},
			expected:    string(validCert1) + string(validCert2),
			expectError: false,
		},
		{
			name: "Mixture of valid and invalid certificates",
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cm1",
						Namespace: "test",
					},
					Data: map[string]string{
						"valid.pem":     string(validCert1),
						"invalid.pem":   string(invalidCertData),
						"valid2.pem":    string(validCert2),
						"nonCert.pem":   string(invalidTypeData),
						"empty.pem":     string(emptyData),
						"multiCert.pem": string(validCert3),
					},
				},
			},
			// We expect the certs in the alphabetical order of the keys
			expected:    string(validCert3) + string(validCert1) + string(validCert2),
			expectError: false,
		},
		{
			name: "Empty ConfigMap",
			configMaps: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "empty",
						Namespace: "test",
					},
					Data: map[string]string{},
				},
			},
			expected:    "",
			expectError: false,
		},
		{
			name:        "No ConfigMaps",
			configMaps:  []*corev1.ConfigMap{},
			expected:    "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := combineValidTrustBundles(tt.configMaps, &buf)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Did not expect error but got: %v", err)
			}

			// For valid certs, verify the actual content and order
			if !tt.expectError {
				result := buf.Bytes()
				expected := []byte(tt.expected)

				// Parse expected certificates
				expectedCerts := parseCertsFromPEM(t, expected)
				// Parse result certificates
				resultCerts := parseCertsFromPEM(t, result)

				// Check count
				if len(expectedCerts) != len(resultCerts) {
					t.Errorf("Expected %d certificates, got %d",
						len(expectedCerts), len(resultCerts))
				}

				// Check order and equality
				for i := 0; i < len(expectedCerts) && i < len(resultCerts); i++ {
					if !expectedCerts[i].Equal(resultCerts[i]) {
						t.Errorf("Certificate at index %d does not match expected certificate", i)
					}
				}
			}
		})
	}
}
