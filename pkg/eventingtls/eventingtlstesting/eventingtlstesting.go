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

package eventingtlstesting

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret/fake"

	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/kncloudevents"
)

var (
	CA  []byte
	Key []byte
	Crt []byte
)

const (
	IssuerKind = "ClusterIssuer"
	IssuerName = "knative-eventing-ca-issuer"
)

func init() {
	CA, Key, Crt = loadCerts()
}

func StartServer(ctx context.Context, t *testing.T, port int, handler http.Handler, receiverOptions ...kncloudevents.HTTPEventReceiverOption) (string, int) {
	secret := types.NamespacedName{
		Namespace: "knative-tests",
		Name:      "tls-secret",
	}

	_ = secretinformer.Get(ctx).Informer().GetStore().Add(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: secret.Namespace,
			Name:      secret.Name,
		},
		Data: map[string][]byte{
			"tls.key": Key,
			"tls.crt": Crt,
		},
		Type: corev1.SecretTypeTLS,
	})

	serverTLSConfig := eventingtls.NewDefaultServerConfig()
	serverTLSConfig.GetCertificate = eventingtls.GetCertificateFromSecret(ctx, secretinformer.Get(ctx), kubeclient.Get(ctx), secret)
	tlsConfig, err := eventingtls.GetTLSServerConfig(serverTLSConfig)
	assert.Nil(t, err)

	receiver := kncloudevents.NewHTTPEventReceiver(port,
		append(receiverOptions,
			kncloudevents.WithTLSConfig(tlsConfig),
		)...,
	)

	go func() {
		err := receiver.StartListen(ctx, handler)
		if err != nil {
			panic(err)
		}
	}()

	return string(CA), receiver.GetPort()
}

func loadCerts() ([]byte, []byte, []byte) {
	caCert, _ /* ca1Key */, srvCert, srvKey := mustChain("Knative-Example-Root-CA")

	return caCert, srvKey, srvCert
}

func mustChain(cn string) ([]byte, []byte, []byte, []byte) {
	caCert, caKey := mustCA(cn)
	srvKey, csr := mustCSR()
	srvCert := mustSign(csr, caCert, caKey)

	return pemCert(caCert.Raw),
		pemKey(caKey),
		pemCert(srvCert),
		pemKey(srvKey)
}

// --- CA (openssl req -x509) ---
func mustCA(cn string) (*x509.Certificate, *rsa.PrivateKey) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			Country:    []string{"US"},
			CommonName: cn,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(1024 * 24 * time.Hour),

		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign |
			x509.KeyUsageCRLSign |
			x509.KeyUsageDigitalSignature,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}

	cert, _ := x509.ParseCertificate(der)
	return cert, key
}

// --- CSR (openssl req -new) ---
func mustCSR() (*rsa.PrivateKey, []byte) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	csrTmpl := &x509.CertificateRequest{
		Subject: pkix.Name{
			Country:      []string{"US"},
			Province:     []string{"YourState"},
			Locality:     []string{"YourCity"},
			Organization: []string{"Example-Certificates"},
			CommonName:   "localhost.local",
		},
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, csrTmpl, key)
	if err != nil {
		panic(err)
	}

	return key, csrDER
}

// --- Sign (openssl x509 -req -extfile domains.ext) ---
func mustSign(csrDER []byte, ca *x509.Certificate, caKey *rsa.PrivateKey) []byte {
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		panic(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      csr.Subject,

		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(1024 * 24 * time.Hour),

		// domains.ext equivalent
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},

		BasicConstraintsValid: true,
		IsCA:                  false,

		KeyUsage: x509.KeyUsageDigitalSignature |
			x509.KeyUsageContentCommitment | // nonRepudiation
			x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDataEncipherment,

		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, csr.PublicKey, caKey)
	if err != nil {
		panic(err)
	}

	return der
}

// --- PEM helpers ---
func pemCert(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: der,
	})
}

func pemKey(key *rsa.PrivateKey) []byte {
	b, _ := x509.MarshalPKCS8PrivateKey(key)
	return pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: b,
	})
}
