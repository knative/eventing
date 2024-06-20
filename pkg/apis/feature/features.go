/*
Copyright 2021 The Knative Authors

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

package feature

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// Flag is a string value which can be either Enabled, Disabled, or Allowed.
type Flag string

const (
	// Enabled turns on an optional behavior.
	Enabled Flag = "Enabled"
	// Disabled turns off an optional behavior.
	Disabled Flag = "Disabled"
	// Allowed neither explicitly disables or enables a behavior.
	// eg. allow a client to control behavior with an annotation or allow a new value through validation.
	Allowed Flag = "Allowed"
	// Strict is only applicable to the TransportEncryption feature.
	// The following applies:
	// - Addressables must not accept events to non-HTTPS endpoints
	// - Addressables must only advertise HTTPS endpoints
	Strict Flag = "Strict"
	// Permissive is only applicable to the TransportEncryption feature.
	// The following applies:
	// - Addressables should accept events at both HTTP and HTTPS endpoints
	// - Addressables should advertise both HTTP and HTTPS endpoints
	// - Producers should prefer to send events to HTTPS endpoints, if available
	Permissive Flag = "Permissive"

	// AuthorizationAllowAll is a value for AuthorizationDefaultMode that indicates to allow all
	// OIDC subjects by default.
	// This configuration is applied when there is no EventPolicy with a "to" referencing a given
	// resource.
	AuthorizationAllowAll Flag = "Allow-All"

	// AuthorizationDenyAll is a value for AuthorizationDefaultMode that indicates to deny all
	// OIDC subjects by default.
	// This configuration is applied when there is no EventPolicy with a "to" referencing a given
	// resource.
	AuthorizationDenyAll Flag = "Deny-All"

	// AuthorizationAllowSameNamespace is a value for AuthorizationDefaultMode that indicates to allow
	// OIDC subjects with the same namespace as a given resource.
	// This configuration is applied when there is no EventPolicy with a "to" referencing a given
	// resource.
	AuthorizationAllowSameNamespace Flag = "Allow-Same-Namespace"
)

// Flags is a map containing all the enabled/disabled flags for the experimental features.
// Missing entry in the map means feature is equal to feature not enabled.
type Flags map[string]Flag

func newDefaults() Flags {
	return map[string]Flag{
		KReferenceGroup:          Disabled,
		DeliveryRetryAfter:       Disabled,
		DeliveryTimeout:          Enabled,
		KReferenceMapping:        Disabled,
		NewTriggerFilters:        Enabled,
		TransportEncryption:      Disabled,
		OIDCAuthentication:       Disabled,
		EvenTypeAutoCreate:       Disabled,
		NewAPIServerFilters:      Disabled,
		AuthorizationDefaultMode: AuthorizationAllowSameNamespace,
	}
}

// IsEnabled returns true if the feature is enabled
func (e Flags) IsEnabled(featureName string) bool {
	return e != nil && e[featureName] == Enabled
}

// IsDisabled returns true if the feature is disabled
func (e Flags) IsDisabled(featureName string) bool {
	return e != nil && e[featureName] == Disabled
}

// IsAllowed returns true if the feature is enabled or allowed
func (e Flags) IsAllowed(featureName string) bool {
	return e.IsEnabled(featureName) || (e != nil && e[featureName] == Allowed)
}

// IsPermissiveTransportEncryption returns true if the TransportEncryption feature is in Permissive mode.
func (e Flags) IsPermissiveTransportEncryption() bool {
	return e != nil && e[TransportEncryption] == Permissive
}

// IsStrictTransportEncryption returns true if the TransportEncryption feature is in Strict mode.
func (e Flags) IsStrictTransportEncryption() bool {
	return e != nil && e[TransportEncryption] == Strict
}

// IsDisabledTransportEncryption returns true if the TransportEncryption feature is in Disabled mode.
func (e Flags) IsDisabledTransportEncryption() bool {
	return e != nil && e[TransportEncryption] == Disabled
}

func (e Flags) IsOIDCAuthentication() bool {
	return e != nil && e[OIDCAuthentication] == Enabled
}

func (e Flags) IsCrossNamespaceEventLinks() bool {
	return e != nil && e[CrossNamespaceEventLinks] == Enabled
}

func (e Flags) IsAuthorizationDefaultModeAllowAll() bool {
	return e != nil && e[AuthorizationDefaultMode] == AuthorizationAllowAll
}

func (e Flags) IsAuthorizationDefaultModeDenyAll() bool {
	return e != nil && e[AuthorizationDefaultMode] == AuthorizationDenyAll
}

func (e Flags) IsAuthorizationDefaultModeSameNamespace() bool {
	return e != nil && e[AuthorizationDefaultMode] == AuthorizationAllowSameNamespace
}

func (e Flags) String() string {
	return fmt.Sprintf("%+v", map[string]Flag(e))
}

func (e Flags) NodeSelector() map[string]string {
	// Check if NodeSelector is not nil
	if e == nil {
		return map[string]string{}
	}

	nodeSelectorMap := make(map[string]string)

	for k, v := range e {
		if strings.Contains(k, NodeSelectorLabel) {
			key := strings.TrimPrefix(k, NodeSelectorLabel)
			value := strings.TrimSpace(string(v))
			nodeSelectorMap[key] = value
		}
	}

	return nodeSelectorMap
}

// NewFlagsConfigFromMap creates a Flags from the supplied Map
func NewFlagsConfigFromMap(data map[string]string) (Flags, error) {
	flags := newDefaults()

	for k, v := range data {
		if strings.HasPrefix(k, "_") {
			// Ignore all the keys starting with _
			continue
		}
		sanitizedKey := strings.TrimSpace(k)
		if strings.EqualFold(v, string(Allowed)) {
			flags[sanitizedKey] = Allowed
		} else if strings.EqualFold(v, string(Disabled)) {
			flags[sanitizedKey] = Disabled
		} else if strings.EqualFold(v, string(Enabled)) {
			flags[sanitizedKey] = Enabled
		} else if sanitizedKey == TransportEncryption && strings.EqualFold(v, string(Permissive)) {
			flags[sanitizedKey] = Permissive
		} else if sanitizedKey == TransportEncryption && strings.EqualFold(v, string(Strict)) {
			flags[sanitizedKey] = Strict
		} else if sanitizedKey == AuthorizationDefaultMode && strings.EqualFold(v, string(AuthorizationAllowAll)) {
			flags[sanitizedKey] = AuthorizationAllowAll
		} else if sanitizedKey == AuthorizationDefaultMode && strings.EqualFold(v, string(AuthorizationDenyAll)) {
			flags[sanitizedKey] = AuthorizationDenyAll
		} else if sanitizedKey == AuthorizationDefaultMode && strings.EqualFold(v, string(AuthorizationAllowSameNamespace)) {
			flags[sanitizedKey] = AuthorizationAllowSameNamespace
		} else if strings.Contains(k, NodeSelectorLabel) {
			flags[sanitizedKey] = Flag(v)
		} else {
			return flags, fmt.Errorf("cannot parse the feature flag '%s' = '%s'", k, v)
		}
	}

	return flags, nil
}

// NewFlagsConfigFromConfigMap creates a Flags from the supplied configMap
func NewFlagsConfigFromConfigMap(config *corev1.ConfigMap) (Flags, error) {
	return NewFlagsConfigFromMap(config.Data)
}
