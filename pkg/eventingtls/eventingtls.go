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
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
	"sync/atomic"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	coreinformersv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

const (
	// TLSKey is the key in the TLS secret for the private key of TLS servers
	TLSKey = "tls.key"
	// TLSCrt is the key in the TLS secret for the public key of TLS servers
	TLSCrt = "tls.crt"
	// DefaultMinTLSVersion is the default minimum TLS version for servers and clients.
	DefaultMinTLSVersion = tls.VersionTLS12
)

type ClientConfig struct {
	// CACerts are Certification Authority (CA) certificates in PEM format
	// according to https://www.rfc-editor.org/rfc/rfc7468.
	CACerts *string
}

type ServerConfig struct {
	// GetCertificate returns a Certificate based on the given
	// ClientHelloInfo. It will only be called if the client supplies SNI
	// information or if Certificates is empty.
	//
	// If GetCertificate is nil or returns nil, then the certificate is
	// retrieved from NameToCertificate. If NameToCertificate is nil, the
	// best element of Certificates will be used.
	GetCertificate func(*tls.ClientHelloInfo) (*tls.Certificate, error)
}

// GetCertificate returns a Certificate based on the given
// ClientHelloInfo. It will only be called if the client supplies SNI
// information or if Certificates is empty.
//
// If GetCertificate is nil or returns nil, then the certificate is
// retrieved from NameToCertificate. If NameToCertificate is nil, the
// best element of Certificates will be used.
type GetCertificate func(*tls.ClientHelloInfo) (*tls.Certificate, error)

// GetCertificateFromSecret returns a GetCertificate function that will automatically return
// the latest certificate that is present in the provided secret.
//
// The secret is expected to have at least 2 keys in data: see TLSKey and TLSCrt constants for
// knowing the key names.
func GetCertificateFromSecret(ctx context.Context, informer coreinformersv1.SecretInformer, kube kubernetes.Interface, secret types.NamespacedName) GetCertificate {

	certHolder := atomic.Value{}

	logger := logging.FromContext(ctx).Desugar().
		With(zap.String("tls.secret", secret.String()))

	store := func(obj interface{}) {
		s, ok := obj.(*corev1.Secret)
		if !ok {
			return
		}
		crt, crtOk := s.Data[TLSCrt]
		key, keyOk := s.Data[TLSKey]
		if !crtOk || !keyOk {
			logger.Debug("Missing " + TLSCrt + " or " + TLSKey + " in the secret.data")
			return
		}

		logger.Debug("Loading key pair")

		certificate, err := tls.X509KeyPair(crt, key)
		if err != nil {
			logger.Error("Failed to create x.509 key pair", zap.Error(err))
			return
		}

		logger.Debug("certificate stored")
		certHolder.Store(&certificate)
	}

	informer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(secret.Namespace, secret.Name),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: store,
			UpdateFunc: func(_, newObj interface{}) {
				store(newObj)
			},
			DeleteFunc: nil,
		},
	})

	// Store the current value so that we have certHolder initialized.
	firstValue, err := informer.Lister().Secrets(secret.Namespace).Get(secret.Name)
	if err != nil {
		// Try to get the secret from the API Server when the lister failed.
		firstValue, err = kube.CoreV1().Secrets(secret.Namespace).Get(ctx, secret.Name, metav1.GetOptions{})
		if err != nil {
			logger.Fatal(err.Error())
		}
	}
	store(firstValue)

	return func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
		cert := certHolder.Load()
		if cert == nil {
			return nil, nil
		}
		return cert.(*tls.Certificate), nil
	}
}

// NewDefaultClientConfig returns a default ClientConfig.
func NewDefaultClientConfig() ClientConfig {
	return ClientConfig{}
}

// GetTLSClientConfig returns tls.Config based on the given ClientConfig.
func GetTLSClientConfig(config ClientConfig) (*tls.Config, error) {
	pool, err := certPool(config.CACerts)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		RootCAs:    pool,
		MinVersion: DefaultMinTLSVersion,
	}, nil
}

func NewDefaultServerConfig() ServerConfig {
	return ServerConfig{}
}

func GetTLSServerConfig(config ServerConfig) (*tls.Config, error) {
	return &tls.Config{
		MinVersion:     DefaultMinTLSVersion,
		GetCertificate: config.GetCertificate,
	}, nil
}

// IsHttpsSink returns true if the sink has scheme equal to https.
func IsHttpsSink(sink string) bool {
	s, err := apis.ParseURL(sink)
	if err != nil {
		return false
	}
	return strings.EqualFold(s.Scheme, "https")
}

// certPool returns a x509.CertPool with the combined certs from:
// - the system cert pool
// - the given CA certificates
func certPool(caCerts *string) (*x509.CertPool, error) {
	p, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}

	if caCerts == nil || *caCerts == "" {
		return p, nil
	}

	if ok := p.AppendCertsFromPEM([]byte(*caCerts)); !ok {
		return p, fmt.Errorf("failed to append CA certs from PEM")
	}

	return p, nil
}
