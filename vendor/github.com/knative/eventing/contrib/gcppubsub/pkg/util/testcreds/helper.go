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

package testcreds

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// InvalidCredsError is the error string returned when using MakeSecretWithInvalidCreds().
	InvalidCredsError = "unexpected end of JSON input"
)

var (
	// Secret is a reference to the Secret created by either MakeSecretWithCreds() and
	// MakeSecretWithInvalidCreds().
	Secret = &v1.ObjectReference{
		APIVersion: "v1",
		Kind:       "Secret",
		Namespace:  "secret-namespace",
		Name:       "secret-name",
	}

	// SecretKey is the key in the Secret data that contains the JSON credential token.
	SecretKey = "key.json"
)

// MakeSecretWithCreds makes a Secret that can successfully be passed through GetCredentials.
// Only tests should be calling this.
func MakeSecretWithCreds() *v1.Secret {
	return &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: Secret.APIVersion,
			Kind:       Secret.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      Secret.Name,
			Namespace: Secret.Namespace,
		},
		Data: map[string][]byte{
			SecretKey: []byte(`{"type": "authorized_user"}`),
		},
	}
}

// MakeSecretWithInvalidCreds makes a Secret that fails to pass GetCredential. Only tests should be
// calling this.
func MakeSecretWithInvalidCreds() *v1.Secret {
	secret := MakeSecretWithCreds()
	secret.Data[SecretKey] = []byte("")
	return secret
}
