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
	"fmt"
	"net/http"
	"strings"
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
