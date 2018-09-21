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
)

// Validate validates the ClusterProvisioner resource.
func (p *ClusterProvisioner) Validate() *apis.FieldError {
	return p.Spec.Validate().ViaField("spec")
}

// Validate validates the ClusterProvisioner spec
func (ps *ClusterProvisionerSpec) Validate() *apis.FieldError {
	if equality.Semantic.DeepEqual(ps, &ClusterProvisionerSpec{}) {
		return apis.ErrMissingField("reconciles")
	}
	var errs *apis.FieldError
	if ps.Reconciles.Kind == "" {
		errs = errs.Also(apis.ErrMissingField("kind").ViaField("reconciles"))
	}
	if ps.Reconciles.Group == "" {
		errs = errs.Also(apis.ErrMissingField("group").ViaField("reconciles"))
	}

	return errs
}
