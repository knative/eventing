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

package main

import (
	"crypto/tls"
	"testing"

	"knative.dev/eventing/pkg/eventingtls"
	reconcilertesting "knative.dev/pkg/reconciler/testing"

	// Fake injection informers and clients
	_ "knative.dev/pkg/client/injection/kube/client/fake"
	_ "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret/fake"
)

func TestGetServerTLSConfig(t *testing.T) {
	t.Setenv("SYSTEM_NAMESPACE", "knative-eventing")

	ctx, _ := reconcilertesting.SetupFakeContext(t)

	tlsConfig, err := getServerTLSConfig(ctx)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if tlsConfig == nil {
		t.Fatal("expected non-nil TLS config")
	}

	if tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("want MinVersion TLS 1.2 (%d), got %d", tls.VersionTLS12, tlsConfig.MinVersion)
	}

	if tlsConfig.GetCertificate == nil {
		t.Fatal("expected GetCertificate to be set")
	}
}

func TestGetServerTLSConfigUsesSecretName(t *testing.T) {
	if eventingtls.RequestReplyServerTLSSecretName != "request-reply-server-tls" {
		t.Fatalf("expected secret name %q, got %q", "request-reply-server-tls", eventingtls.RequestReplyServerTLSSecretName)
	}
}
