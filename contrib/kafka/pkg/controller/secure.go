package controller

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
)

// NewTLSConfig configures TLS Config by adding ca cert.
// If ca is passed by empty string, it returns &tls.Config object instead of nil.
func NewTLSConfig(caCert string) (*tls.Config, error) {
	tlsConfig := tls.Config{}

	if len(caCert) == 0 {
		// No CA is passed, TLS uses the system's root CA set.
		return &tlsConfig, nil
	}

	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM([]byte(caCert))
	if !ok {
		return nil, fmt.Errorf("Unable to add ca cert.")
	}
	tlsConfig.RootCAs = caCertPool
	tlsConfig.BuildNameToCertificate()

	return &tlsConfig, nil
}
