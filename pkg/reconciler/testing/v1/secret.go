/*
Copyright 2020 The Knative Authors

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

package testing

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretOption enables further configuration of a Secret.
type SecretOption func(secret *v1.Secret)

// NewSecret creates a new Secret.
func NewSecret(name, namespace string, o ...SecretOption) *v1.Secret {
	cm := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	for _, opt := range o {
		opt(cm)
	}
	return cm
}

func WithSecretData(data map[string][]byte) SecretOption {
	return func(cm *v1.Secret) {
		cm.Data = data
	}
}
