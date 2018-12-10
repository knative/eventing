/*
Copyright 2018 The Knative Authors

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

package util

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"cloud.google.com/go/pubsub"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	"golang.org/x/oauth2/google"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// GetCredentials gets GCP credentials from a secretRef. The credentials must be stored in JSON format
// in the secretRef.
func GetCredentials(ctx context.Context, client client.Client, secretRef *v1.ObjectReference, key string) (*google.Credentials, error) {
	secret := &v1.Secret{}
	err := client.Get(ctx, types.NamespacedName{Namespace: secretRef.Namespace, Name: secretRef.Name}, secret)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to read the secretRef", zap.Any("secretRef", secretRef))
		return nil, err
	}

	bytes, present := secret.Data[key]
	if !present {
		logging.FromContext(ctx).Error("Secret did not contain the key", zap.String("key", key))
		return nil, fmt.Errorf("secretRef did not contain the key '%s'", key)
	}

	creds, err := google.CredentialsFromJSON(ctx, bytes, pubsub.ScopePubSub)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to create the GCP credential", zap.Error(err))
		return nil, err
	}
	return creds, nil
}
