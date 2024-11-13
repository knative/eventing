/*
Copyright 2024 The Knative Authors

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

package v1alpha1

type Auth struct {
	// Auth Secret
	Secret *Secret `json:"secret,omitempty"`

	// AccessKey is the AWS access key ID.
	AccessKey string `json:"accessKey,omitempty"`

	// SecretKey is the AWS secret access key.
	SecretKey string `json:"secretKey,omitempty"`
}

func (a *Auth) HasAuth() bool {
	return a != nil && a.Secret != nil &&
		a.Secret.Ref != nil && a.Secret.Ref.Name != ""
}

type Secret struct {
	// Secret reference for SASL and SSL configurations.
	Ref *SecretReference `json:"ref,omitempty"`
}

type SecretReference struct {
	// Secret name.
	Name string `json:"name"`
}
