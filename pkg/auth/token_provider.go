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
	"time"

	"go.uber.org/zap"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/cache"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
)

const (
	TokenExpirationTime  = time.Hour
	expirationBufferTime = 5 * time.Minute
)

type OIDCTokenProvider struct {
	logger     *zap.SugaredLogger
	kubeClient kubernetes.Interface
	tokenCache cache.Expiring
}

func NewOIDCTokenProvider(ctx context.Context) *OIDCTokenProvider {
	tokenProvider := &OIDCTokenProvider{
		logger:     logging.FromContext(ctx).With("component", "oidc-token-provider"),
		kubeClient: kubeclient.Get(ctx),
		tokenCache: *cache.NewExpiring(),
	}

	return tokenProvider
}

// GetJWT returns a JWT from the given service account for the given audience.
func (c *OIDCTokenProvider) GetJWT(serviceAccount types.NamespacedName, audience string) (string, error) {
	if val, ok := c.tokenCache.Get(cacheKey(serviceAccount, audience)); ok {
		return val.(string), nil
	}

	// if not found in cache: request new token
	return c.GetNewJWT(serviceAccount, audience)
}

// GetNewJWT returns a new JWT from the given service account for the given audience without using the token cache.
func (c *OIDCTokenProvider) GetNewJWT(serviceAccount types.NamespacedName, audience string) (string, error) {
	// request new token
	tokenRequest := authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			Audiences:         []string{audience},
			ExpirationSeconds: pointer.Int64(int64(TokenExpirationTime.Seconds())),
		},
	}

	tokenRequestResponse, err := c.kubeClient.
		CoreV1().
		ServiceAccounts(serviceAccount.Namespace).
		CreateToken(context.TODO(), serviceAccount.Name, &tokenRequest, metav1.CreateOptions{})

	if err != nil {
		return "", fmt.Errorf("could not request a token for %s: %w", serviceAccount, err)
	}

	// we need a duration until this token expires, use the expiry time - (now + 5min)
	// this gives us a buffer so that it doesn't expire between when we retrieve it and when we use it
	expiryTtl := tokenRequestResponse.Status.ExpirationTimestamp.Time.Sub(time.Now().Add(expirationBufferTime))

	c.tokenCache.Set(cacheKey(serviceAccount, audience), tokenRequestResponse.Status.Token, expiryTtl)

	return tokenRequestResponse.Status.Token, nil
}

func cacheKey(serviceAccount types.NamespacedName, audience string) string {
	return fmt.Sprintf("%s/%s/%s", serviceAccount.Namespace, serviceAccount.Name, audience)
}
