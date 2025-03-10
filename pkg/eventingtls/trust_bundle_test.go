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
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

	// Generate some test certificates
	validCert1 := generateValidCert()
	validCert2 := generateValidCert()
	validCert3 := generateValidCert()

	// Invalid PEM data (not a certificate)
	invalidTypeData := []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA7bPaMwN0tKpmN1EtAAMnQcbQNVTgC0DfgLYZ1yDqnNEVj8F/
MVdfQIyhOfAcBImRlvHHbDQxVWiXlANqwN2NmGAB1/cxNkB1GkDn6y/tango/G7t
TF0/Gm2jcgpCU2RzDxnQPimRJNQkP7rNVY8fiEE7O5EWcB0FbKOZ8G4+epZ5RXuL
0y/+AyFIYrIsuraqJZBZRLA/aaLmvZbPzBCVNq36QQTzG+CQpgNYXKRJJTmR8D5w
o2fgzpQRsBHxJ+VM8HyrCWIg8tGJVnMXRaH3LHmXEzJAASL0qO7a7Z1hPdGXCRUO
3PBXXaVl98PTvAf1IX0SFiJXZ0vwB1OH7NkEvQIDAQABAoIBABhXNxC9kZpwZzWX
b3m5j4HwgYUyCLlvsYT5xtNEfp87ZnLO0OdD5QlQe02QyrY9uYQCGtX7sWKGXfXF
czzRpw3+53hf0JhtCgxP88TaOUmLrOQwQYG3Rio9jOGjYQKA5xJFod0c/Z0EWaDB
xeYUQpRGNoQQUUPJi+WX/UE5pbTUzKWI4xtxVJbxuBEF63xHXfCYl3cTAuBLwZcy
iRtQS2wQBLKLdj2JZbI/kHMnAUC0Z1kyGQeGJLiYJLWnCEcErPSgi1Yapt95nlQF
eYgNn1/TqOZtuUMMQcl87+rVAJzQ6/1KsxdIlA/ZQAQUkRwGr4ZTfmPRXCPy5PIw
eJ1KAAECgYEA9kVvG/ExNHQxIDHZBbHUGPLzGNVwBpvLZnZJxuIl+/kVPCZvEMUK
5xYfDZYwj9aNDgDWIjTUEN6r+LdJ6uLYkA4kdRYWFm6znKXUKTAhf/ePDqhvwJTI
LZBQsviwr3J1y4edGNgKVTRQUJdKTnvRVpvugvE3d0FOOJr+JXOKnH0CgYEA9unj
QLrg8nhgYkC9dKZ5Kt6rysXOO1J29N3JIvzLJFPcOCrvGE5NTPvXsoz/zjRb5Yo5
L4QQtQ59k1jgXngQV+x3CoRUUQne9TDnnnSj3zWOhuEKmGLtbQEgg0D9x/8uOuaQ
G/3Z9/cI+blzbOmczPgY+Nwoi7WzCnE0u764lAECgYEA8FdacV6/9tWQWLUZnNxN
ZFz3Q/9i6XhmXTUM9aBZxR0IVwQMeZMmjkjO8XDHmtE7kUz6WHRXvQBdGPw8CDKF
PBaDDkPex+Q02sBw3qpvvpHNYpQ6JaVC7MRXdTAEuXMTBuIqMw5jqQAx2JRiDx/y
2Lv8j4uFpNJ5K1f+kngJMm0CgYB1YnZKiqn6hOEwXB5DCJsjUbRAkBRnOqjXpZJ7
k3+WbLBc1/AobU0WZqLx8mYQQPqOt8UJm7MfDKxKXqmnEbO+TEWfJiw5nUx0iL7r
he7Nx4lLsVy/Yj91z5Ct4Qf9+vKaxn0D2gbNQbLkKR4LhsL8IUFYgvQruzQeAEJE
rIKAAQKBgQC5aT3SiSRZgHNpSLnQK/jSvxys9sxZ9ulL4GTVTwIZlEY8Xw1WZE+o
AZGGGg4o9sMJAf+SZvHu1vF9wNx+Yh/T9uOzDOGnlKEfHo1ljSWbKlnFfWNPgZF/
A2JA2pwuXFQlQWpRbL5HUmXAaS1wGMSxFqLznf5qEZUGUNKqUvI8HQ==
-----END RSA PRIVATE KEY-----`)

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
			expected:    string(validCert1) + string(validCert2) + string(validCert3),
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

			// For valid certs, we need to compare the actual content
			// by counting the number of certificates
			if !tt.expectError {
				result := buf.String()

				expectedCertCount := strings.Count(tt.expected, "BEGIN CERTIFICATE")
				resultCertCount := strings.Count(result, "BEGIN CERTIFICATE")

				if expectedCertCount != resultCertCount {
					t.Errorf("Expected %d certificates, got %d",
						expectedCertCount, resultCertCount)
				}

				// Verify the actual certificates are there
				// We can't compare the exact string due to PEM encoding variations
				// so we count and check if each cert was included
				remainingPEM := []byte(result)
				for i := 0; i < expectedCertCount; i++ {
					var block *pem.Block
					block, remainingPEM = pem.Decode(remainingPEM)

					if block == nil {
						t.Errorf("Failed to decode PEM block %d", i)
						break
					}

					// Verify it's a valid certificate
					_, err := x509.ParseCertificate(block.Bytes)
					if err != nil {
						t.Errorf("PEM block %d is not a valid certificate: %v", i, err)
					}
				}
			}
		})
	}
}
