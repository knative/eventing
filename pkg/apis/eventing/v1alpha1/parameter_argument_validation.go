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
	"github.com/knative/pkg/apis"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/validation"
)

// Validate validates the ParameterSpec resource.
func (ps *ParameterSpec) Validate() *apis.FieldError {
	if equality.Semantic.DeepEqual(ps, &ParameterSpec{}) {
		return apis.ErrMissingField("name")
	}

	switch {
	case ps.Default != "" && ps.DefaultFrom != nil:
		return apis.ErrMultipleOneOf("default", "defaultFrom")
	case ps.Default != "":
		// not sure how to validate the value.
	case ps.DefaultFrom != nil:
		if err := ps.DefaultFrom.Validate(); err != nil {
			return err.ViaField("defaultFrom")
		}
	default:
		return apis.ErrMissingOneOf("default", "defaultFrom")
	}

	return nil
}

// Validate validates the ArgumentValueReference resource.
func (arg *ArgumentValueReference) Validate() *apis.FieldError {
	switch {
	case arg.SecretKeyRef != nil && arg.ConfigMapRef != nil:
		return apis.ErrMultipleOneOf("secretKeyRef", "configMapRef")
	case arg.SecretKeyRef != nil:
		if err := arg.SecretKeyRef.Validate(); err != nil {
			return err.ViaField("secretKeyRef")
		}
	case arg.ConfigMapRef != nil:
		if err := arg.ConfigMapRef.Validate(); err != nil {
			return err.ViaField("configMapRef")
		}
	default:
		return apis.ErrMissingOneOf("secretKeyRef", "configMapRef")
	}
	return nil
}

// Validate validates the SecretKeyReference resource.
func (sk *SecretKeyReference) Validate() *apis.FieldError {
	if equality.Semantic.DeepEqual(sk, &ParameterSpec{}) {
		return apis.ErrMissingField("name", "key")
	}

	if err := isQualifiedName(sk.Name, true); err != nil {
		return err.ViaField("name")
	}

	if err := isQualifiedName(sk.Key, true); err != nil {
		return err.ViaField("key")
	}

	return nil
}

// Validate validates the ConfigMapReference resource.
func (cm *ConfigMapReference) Validate() *apis.FieldError {
	if equality.Semantic.DeepEqual(cm, &ParameterSpec{}) {
		return apis.ErrMissingField("name", "key")
	}

	if err := isQualifiedName(cm.Name, true); err != nil {
		return err.ViaField("name")
	}

	if err := isQualifiedName(cm.Key, true); err != nil {
		return err.ViaField("key")
	}

	return nil
}

func isQualifiedName(name string, required bool) *apis.FieldError {
	if required && name == "" {
		return apis.ErrMissingField(apis.CurrentField)
	}
	if errs := validation.IsQualifiedName(name); len(errs) > 0 {
		return apis.ErrInvalidValue(name, apis.CurrentField)
	}
	return nil
}
