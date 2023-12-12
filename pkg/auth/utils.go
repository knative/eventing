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
	"fmt"
	"net/http"
	nethttp "net/http"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
)

const (
	AuthHeaderKey = "Authorization"
)

// GetJWTFromHeader Returns the JWT from the Authorization header
func GetJWTFromHeader(header http.Header) string {
	authHeader := header.Get(AuthHeaderKey)
	if authHeader == "" {
		return ""
	}

	return strings.TrimPrefix(authHeader, "Bearer ")
}

// SetAuthHeader sets Authorization header with the given JWT
func SetAuthHeader(jwt string, header http.Header) {
	header.Set(AuthHeaderKey, fmt.Sprintf("Bearer %s", jwt))
}

// GetJWTExpiry returns the expiry time of the token in UTC
func GetJWTExpiry(token string) (time.Time, error) {
	t, err := jwt.ParseSigned(token)
	if err != nil {
		return time.Time{}, err
	}

	var claims jwt.Claims
	if err := t.UnsafeClaimsWithoutVerification(&claims); err != nil {
		return time.Time{}, err
	}

	if claims.Expiry == nil {
		return time.Time{}, fmt.Errorf("no expiry set in JWT")
	}

	return claims.Expiry.Time(), nil
}

// VerifyJWTFromRequest will verify the incoming request contains the correct JWT token
func VerifyJWTFromRequest(ctx context.Context, tokenVerifier *OIDCTokenVerifier, r *http.Request, audience *string, response http.ResponseWriter) error {
	token := GetJWTFromHeader(r.Header)
	if token == "" {
		response.WriteHeader(nethttp.StatusUnauthorized)
		return fmt.Errorf("no JWT token found in request")
	}

	if audience == nil {
		response.WriteHeader(nethttp.StatusInternalServerError)
		return fmt.Errorf("no audience is provided")
	}

	if _, err := tokenVerifier.VerifyJWT(ctx, token, *audience); err != nil {
		response.WriteHeader(http.StatusUnauthorized)
		return fmt.Errorf("failed to verify JWT: %w", err)
	}

	return nil
}
