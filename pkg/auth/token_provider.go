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

	"go.uber.org/zap"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
)

type OIDCTokenProvider struct {
	logger     *zap.SugaredLogger
	kubeClient kubernetes.Interface
}

func NewOIDCTokenProvider(ctx context.Context) *OIDCTokenProvider {
	tokenProvider := &OIDCTokenProvider{
		logger:     logging.FromContext(ctx).With("component", "oidc-token-provider"),
		kubeClient: kubeclient.Get(ctx),
	}

	return tokenProvider
}

// GetJWT returns a JWT from the given service account for the given audience.
func (c *OIDCTokenProvider) GetJWT(serviceAccount types.NamespacedName, audience string) (string, error) {
	// TODO: check the cache

	// if not found in cache: request new token
	tokenRequest := authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			Audiences: []string{audience},
		},
	}

	tokenRequestResponse, err := c.kubeClient.
		CoreV1().
		ServiceAccounts(serviceAccount.Namespace).
		CreateToken(context.TODO(), serviceAccount.Name, &tokenRequest, metav1.CreateOptions{})

	if err != nil {
		return "", fmt.Errorf("could not request a token for %s: %w", serviceAccount, err)
	}

	return tokenRequestResponse.Status.Token, nil
}
