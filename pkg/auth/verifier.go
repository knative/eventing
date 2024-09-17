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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"

	"github.com/cloudevents/sdk-go/v2/binding"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/coreos/go-oidc/v3/oidc"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/feature"
	listerseventingv1alpha1 "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const (
	kubernetesOIDCDiscoveryBaseURL = "https://kubernetes.default.svc"
)

type Verifier struct {
	logger            *zap.SugaredLogger
	restConfig        *rest.Config
	provider          *oidc.Provider
	eventPolicyLister v1alpha1.EventPolicyLister
}

type IDToken struct {
	Issuer          string
	Audience        []string
	Subject         string
	Expiry          time.Time
	IssuedAt        time.Time
	AccessTokenHash string
}

func NewVerifier(ctx context.Context, eventPolicyLister listerseventingv1alpha1.EventPolicyLister) *Verifier {
	tokenHandler := &Verifier{
		logger:            logging.FromContext(ctx).With("component", "oidc-token-handler"),
		restConfig:        injection.GetConfig(ctx),
		eventPolicyLister: eventPolicyLister,
	}

	if err := tokenHandler.initOIDCProvider(ctx); err != nil {
		tokenHandler.logger.Error(fmt.Sprintf("could not initialize provider. You can ignore this message, when the %s feature is disabled", feature.OIDCAuthentication), zap.Error(err))
	}

	return tokenHandler
}

// VerifyRequest verifies AuthN and AuthZ in the request. On verification errors, it sets the
// responses HTTP status and returns an error
func (v *Verifier) VerifyRequest(ctx context.Context, features feature.Flags, requiredOIDCAudience *string, resourceNamespace string, policyRefs []duckv1.AppliedEventPolicyRef, req *http.Request, resp http.ResponseWriter) error {
	if !features.IsOIDCAuthentication() {
		return nil
	}

	idToken, err := v.verifyAuthN(ctx, requiredOIDCAudience, req, resp)
	if err != nil {
		return fmt.Errorf("authentication of request could not be verified: %w", err)
	}

	err = v.verifyAuthZ(ctx, features, idToken, resourceNamespace, policyRefs, req, resp)
	if err != nil {
		return fmt.Errorf("authorization of request could not be verified: %w", err)
	}

	return nil
}

// VerifyRequestFromSubject verifies AuthN and AuthZ in the request.
// In the AuthZ part it checks if the request comes from the given allowedSubject.
// On verification errors, it sets the responses HTTP status and returns an error.
// This method is similar to VerifyRequest() except that VerifyRequestFromSubject()
// verifies in the AuthZ part that the request comes from a given subject.
func (v *Verifier) VerifyRequestFromSubject(ctx context.Context, features feature.Flags, requiredOIDCAudience *string, allowedSubject string, req *http.Request, resp http.ResponseWriter) error {
	if !features.IsOIDCAuthentication() {
		return nil
	}

	idToken, err := v.verifyAuthN(ctx, requiredOIDCAudience, req, resp)
	if err != nil {
		return fmt.Errorf("authentication of request could not be verified: %w", err)
	}

	if idToken.Subject != allowedSubject {
		resp.WriteHeader(http.StatusForbidden)
		return fmt.Errorf("token is from subject %q, but only %q is allowed", idToken.Subject, allowedSubject)
	}

	return nil
}

// verifyAuthN verifies if the incoming request contains a correct JWT token
func (v *Verifier) verifyAuthN(ctx context.Context, audience *string, req *http.Request, resp http.ResponseWriter) (*IDToken, error) {
	token := GetJWTFromHeader(req.Header)
	if token == "" {
		resp.WriteHeader(http.StatusUnauthorized)
		return nil, fmt.Errorf("no JWT token found in request")
	}

	if audience == nil {
		resp.WriteHeader(http.StatusInternalServerError)
		return nil, fmt.Errorf("no audience is provided")
	}

	idToken, err := v.verifyJWT(ctx, token, *audience)
	if err != nil {
		resp.WriteHeader(http.StatusUnauthorized)
		return nil, fmt.Errorf("failed to verify JWT: %w", err)
	}

	return idToken, nil
}

// verifyAuthZ verifies if the given idToken is allowed by the resources eventPolicyStatus
func (v *Verifier) verifyAuthZ(ctx context.Context, features feature.Flags, idToken *IDToken, resourceNamespace string, policyRefs []duckv1.AppliedEventPolicyRef, req *http.Request, resp http.ResponseWriter) error {
	if len(policyRefs) > 0 {
		req, err := copyRequest(req)
		if err != nil {
			resp.WriteHeader(http.StatusInternalServerError)
			return fmt.Errorf("failed to copy request body: %w", err)
		}

		message := cehttp.NewMessageFromHttpRequest(req)
		defer message.Finish(nil)

		event, err := binding.ToEvent(ctx, message)
		if err != nil {
			resp.WriteHeader(http.StatusInternalServerError)
			return fmt.Errorf("failed to decode event from request: %w", err)
		}

		subjectsWithFiltersFromApplyingPolicies := []subjectsWithFilters{}
		for _, p := range policyRefs {
			policy, err := v.eventPolicyLister.EventPolicies(resourceNamespace).Get(p.Name)
			if err != nil {
				resp.WriteHeader(http.StatusInternalServerError)
				return fmt.Errorf("failed to get eventPolicy: %w", err)
			}

			subjectsWithFiltersFromApplyingPolicies = append(subjectsWithFiltersFromApplyingPolicies, subjectsWithFilters{subjects: policy.Status.From, filters: policy.Spec.Filters})
		}

		if !SubjectAndFiltersPass(ctx, idToken.Subject, subjectsWithFiltersFromApplyingPolicies, event, v.logger) {
			resp.WriteHeader(http.StatusForbidden)
			return fmt.Errorf("token is from subject %q, but only %#v are part of applying event policies", idToken.Subject, subjectsWithFiltersFromApplyingPolicies)
		}

		return nil
	} else {
		if features.IsAuthorizationDefaultModeDenyAll() {
			resp.WriteHeader(http.StatusForbidden)
			return fmt.Errorf("no event policies apply for resource and %s is set to %s", feature.AuthorizationDefaultMode, feature.AuthorizationDenyAll)

		} else if features.IsAuthorizationDefaultModeSameNamespace() {
			if !strings.HasPrefix(idToken.Subject, fmt.Sprintf("%s:%s:", kubernetesServiceAccountPrefix, resourceNamespace)) {
				resp.WriteHeader(http.StatusForbidden)
				return fmt.Errorf("no policies apply for resource. %s is set to %s, but token is from subject %q, which is not part of %q namespace", feature.AuthorizationDefaultMode, feature.AuthorizationDenyAll, idToken.Subject, resourceNamespace)
			}

			return nil
		}
		// else: allow all
	}

	return nil
}

// verifyJWT verifies the given JWT for the expected audience and returns the parsed ID token.
func (v *Verifier) verifyJWT(ctx context.Context, jwt, audience string) (*IDToken, error) {
	if v.provider == nil {
		return nil, fmt.Errorf("provider is nil. Is the OIDC provider config correct?")
	}

	verifier := v.provider.Verifier(&oidc.Config{
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

func (v *Verifier) initOIDCProvider(ctx context.Context) error {
	discovery, err := v.getKubernetesOIDCDiscovery()
	if err != nil {
		return fmt.Errorf("could not load Kubernetes OIDC discovery information: %w", err)
	}

	if discovery.Issuer != kubernetesOIDCDiscoveryBaseURL {
		// in case we have another issuer as the api server:
		ctx = oidc.InsecureIssuerURLContext(ctx, discovery.Issuer)
	}

	httpClient, err := v.getHTTPClientForKubeAPIServer()
	if err != nil {
		return fmt.Errorf("could not get HTTP client with TLS certs of API server: %w", err)
	}
	ctx = oidc.ClientContext(ctx, httpClient)

	// get OIDC provider
	v.provider, err = oidc.NewProvider(ctx, kubernetesOIDCDiscoveryBaseURL)
	if err != nil {
		return fmt.Errorf("could not get OIDC provider: %w", err)
	}

	v.logger.Debug("updated OIDC provider config", zap.Any("discovery-config", discovery))

	return nil
}

func (v *Verifier) getHTTPClientForKubeAPIServer() (*http.Client, error) {
	client, err := rest.HTTPClientFor(v.restConfig)
	if err != nil {
		return nil, fmt.Errorf("could not create HTTP client from rest config: %w", err)
	}

	return client, nil
}

func (v *Verifier) getKubernetesOIDCDiscovery() (*openIDMetadata, error) {
	client, err := v.getHTTPClientForKubeAPIServer()
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

// copyRequest makes a copy of the http request which can be consumed as needed, leaving the original request
// able to be consumed as well.
func copyRequest(req *http.Request) (*http.Request, error) {
	// check if we actually need to copy the body, otherwise we can return the original request
	if req.Body == nil || req.Body == http.NoBody {
		return req, nil
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(req.Body); err != nil {
		return nil, fmt.Errorf("failed to read request body while copying it: %w", err)
	}

	if err := req.Body.Close(); err != nil {
		return nil, fmt.Errorf("failed to close original request body ready while copying request: %w", err)
	}

	// set the original request body to be readable again
	req.Body = io.NopCloser(&buf)

	// return a new request with a readable body and same headers as the original
	// we don't need to set any other fields as cloudevents only uses the headers
	// and body to construct the Message/Event.
	return &http.Request{
		Header: req.Header,
		Body:   io.NopCloser(bytes.NewReader(buf.Bytes())),
	}, nil
}

type openIDMetadata struct {
	Issuer        string   `json:"issuer"`
	JWKSURI       string   `json:"jwks_uri"`
	ResponseTypes []string `json:"response_types_supported"`
	SubjectTypes  []string `json:"subject_types_supported"`
	SigningAlgs   []string `json:"id_token_signing_alg_values_supported"`
}

type subjectsWithFilters struct {
	filters  []eventingv1.SubscriptionsAPIFilter
	subjects []string
}
