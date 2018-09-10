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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"encoding/json"
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	"github.com/knative/pkg/apis"
	"github.com/knative/pkg/webhook"
	"k8s.io/apimachinery/pkg/runtime"
)

// Check that ParameterSpec can be validated and can be defaulted.
var _ apis.Validatable = (*ParameterSpec)(nil)
var _ apis.Defaultable = (*ParameterSpec)(nil)

// ParameterSpec allows specification of parameters that optionally have defaults.
type ParameterSpec struct {
	// Name is the unique name of this template parameter.
	// +required
	Name string `json:"name"`

	// Description is A human-readable explanation of this template parameter.
	// +optional
	Description string `json:"description,omitempty"`

	// Required specifies if the parameter is required.
	// If true, default / defaultFrom should not be provided.
	// Defaults to false.
	// +immutable
	// +optional
	Required bool `json:"required,omitempty"`

	// Default value if not provided by arguments.
	// +oneof(Default, DefaultFrom), optional.
	Default string `json:"default,omitempty"`

	// DefaultFrom specifies a Default value retrieved from an existing Secret
	// or ConfigMap if not provided by arguments.
	// +oneof(Default, DefaultFrom), optional.
	DefaultFrom *ArgumentValueReference `json:"defaultFrom,omitempty"`
}

// Check that ArgumentValueReference can be validated and can be defaulted.
var _ apis.Validatable = (*ArgumentValueReference)(nil)
var _ apis.Defaultable = (*ArgumentValueReference)(nil)

// ArgumentValueReference holds references to values to be retrieved from an
// existing Secret or ConfigMap.
type ArgumentValueReference struct {
	// SecretKeyRef is a reference to a value contained in a Secret at the given key.
	// +oneof(SecretKeyRef, ConfigMapRef), required.
	SecretKeyRef *SecretKeyReference `json:"secretKeyRef,omitempty"`

	// ConfigMapRef is a reference to a value contained in a ConfigMap at the given key.
	// +oneof(SecretKeyRef, ConfigMapRef), required.
	ConfigMapRef *ConfigMapReference `json:"configMapRef,omitempty"`
}

// Check that ArgumentValueReference can be validated and can be defaulted.
var _ apis.Validatable = (*SecretKeyReference)(nil)
var _ apis.Defaultable = (*SecretKeyReference)(nil)

// SecretKeyReference references a key of a Secret.
type SecretKeyReference struct {
	// The name of the secret in the resource's namespace to select from.
	// +required
	Name string `json:"name"`

	// The key of the secret to select from.  Must be a valid secret key.
	// +required
	Key string `json:"key"`
}

// Check that ArgumentValueReference can be validated and can be defaulted.
var _ apis.Validatable = (*ConfigMapReference)(nil)
var _ apis.Defaultable = (*ConfigMapReference)(nil)

// ConfigMapReference references a key of a Config Map.
type ConfigMapReference struct {
	// The name of the config map in the resource's namespace to select from.
	// +required
	Name string `json:"name"`

	// The key of the config map to select from.  Must be a valid config map key.
	// +required
	Key string `json:"key"`
}
