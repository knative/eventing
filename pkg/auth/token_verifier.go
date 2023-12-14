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

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const (
	kubernetesOIDCDiscoveryBaseURL = "https://kubernetes.default.svc"
)

type OIDCTokenVerifier struct {
	logger     *zap.SugaredLogger
	restConfig *rest.Config
	provider   *oidc.Provider
}

type IDToken struct {
	Issuer          string
	Audience        []string
	Subject         string
	Expiry          time.Time
	IssuedAt        time.Time
	AccessTokenHash string
}

func NewOIDCTokenVerifier(ctx context.Context) *OIDCTokenVerifier {
	tokenHandler := &OIDCTokenVerifier{
		logger:     logging.FromContext(ctx).With("component", "oidc-token-handler"),
		restConfig: injection.GetConfig(ctx),
	}

	if err := tokenHandler.initOIDCProvider(ctx); err != nil {
		tokenHandler.logger.Error(fmt.Sprintf("could not initialize provider. You can ignore this message, when the %s feature is disabled", feature.OIDCAuthentication), zap.Error(err))
	}

	return tokenHandler
}

// VerifyJWT verifies the given JWT for the expected audience and returns the parsed ID token.
func (c *OIDCTokenVerifier) VerifyJWT(ctx context.Context, jwt, audience string) (*IDToken, error) {
	if c.provider == nil {
		return nil, fmt.Errorf("provider is nil. Is the OIDC provider config correct?")
	}

	verifier := c.provider.Verifier(&oidc.Config{
		ClientID: audience,
	})

	token, err := verifier.Verify(ctx, jwt)
	if err != nil {
		return nil, fmt.Errorf("could not verify JWT: %w", err)
	}

	return &IDToken{
		Issuer:          token.Issuer,
		Audience:        token.Audience,
		Subject:         token.Subject,
		Expiry:          token.Expiry,
		IssuedAt:        token.IssuedAt,
		AccessTokenHash: token.AccessTokenHash,
	}, nil
}

func (c *OIDCTokenVerifier) initOIDCProvider(ctx context.Context) error {
	discovery, err := c.getKubernetesOIDCDiscovery()
	if err != nil {
		return fmt.Errorf("could not load Kubernetes OIDC discovery information: %w", err)
	}

	if discovery.Issuer != kubernetesOIDCDiscoveryBaseURL {
		// in case we have another issuer as the api server:
		ctx = oidc.InsecureIssuerURLContext(ctx, discovery.Issuer)
	}

	httpClient, err := c.getHTTPClientForKubeAPIServer()
	if err != nil {
		return fmt.Errorf("could not get HTTP client with TLS certs of API server: %w", err)
	}
	ctx = oidc.ClientContext(ctx, httpClient)

	// get OIDC provider
	c.provider, err = oidc.NewProvider(ctx, kubernetesOIDCDiscoveryBaseURL)
	if err != nil {
		return fmt.Errorf("could not get OIDC provider: %w", err)
	}

	c.logger.Debug("updated OIDC provider config", zap.Any("discovery-config", discovery))

	return nil
}

func (c *OIDCTokenVerifier) getHTTPClientForKubeAPIServer() (*http.Client, error) {
	client, err := rest.HTTPClientFor(c.restConfig)
	if err != nil {
		return nil, fmt.Errorf("could not create HTTP client from rest config: %w", err)
	}

	return client, nil
}

func (c *OIDCTokenVerifier) getKubernetesOIDCDiscovery() (*openIDMetadata, error) {
	client, err := c.getHTTPClientForKubeAPIServer()
	if err != nil {
		return nil, fmt.Errorf("could not get HTTP client for API server: %w", err)
	}

	resp, err := client.Get(kubernetesOIDCDiscoveryBaseURL + "/.well-known/openid-configuration")
	if err != nil {
		return nil, fmt.Errorf("could not get response: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read response body: %w", err)
	}

	openIdConfig := &openIDMetadata{}
	if err := json.Unmarshal(body, openIdConfig); err != nil {
		return nil, fmt.Errorf("could not unmarshall openid config: %w", err)
	}

	return openIdConfig, nil
}

// VerifyJWTFromRequest will verify the incoming request contains the correct JWT token
func (tokenVerifier *OIDCTokenVerifier) VerifyJWTFromRequest(ctx context.Context, r *http.Request, audience *string, response http.ResponseWriter) error {
	token := GetJWTFromHeader(r.Header)
	if token == "" {
		response.WriteHeader(http.StatusUnauthorized)
		return fmt.Errorf("no JWT token found in request")
	}

	if audience == nil {
		response.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("no audience is provided")
	}

	if _, err := tokenVerifier.VerifyJWT(ctx, token, *audience); err != nil {
		response.WriteHeader(http.StatusUnauthorized)
		return fmt.Errorf("failed to verify JWT: %w", err)
	}

	return nil
}

type openIDMetadata struct {
	Issuer        string   `json:"issuer"`
	JWKSURI       string   `json:"jwks_uri"`
	ResponseTypes []string `json:"response_types_supported"`
	SubjectTypes  []string `json:"subject_types_supported"`
	SigningAlgs   []string `json:"id_token_signing_alg_values_supported"`
}
