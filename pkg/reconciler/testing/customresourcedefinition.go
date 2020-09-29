/*
Copyright 2019 The Knative Authors

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
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CustomResourceDefinitionOption enables further configuration of a CustomResourceDefinition.
type CustomResourceDefinitionOption func(*apiextensionsv1.CustomResourceDefinition)

// NewCustomResourceDefinition creates a CustomResourceDefinition with CustomResourceDefinitionOption.
func NewCustomResourceDefinition(name string, o ...CustomResourceDefinitionOption) *apiextensionsv1.CustomResourceDefinition {
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, opt := range o {
		opt(crd)
	}
	return crd
}

// WithCustomResourceDefinitionLabels sets the CRD's labels.
func WithCustomResourceDefinitionLabels(labels map[string]string) CustomResourceDefinitionOption {
	return func(crd *apiextensionsv1.CustomResourceDefinition) {
		crd.Labels = labels
	}
}

func WithCustomResourceDefinitionVersions(versions []apiextensionsv1.CustomResourceDefinitionVersion) CustomResourceDefinitionOption {
	return func(crd *apiextensionsv1.CustomResourceDefinition) {
		crd.Spec.Versions = versions
	}
}

func WithCustomResourceDefinitionGroup(group string) CustomResourceDefinitionOption {
	return func(crd *apiextensionsv1.CustomResourceDefinition) {
		crd.Spec.Group = group
	}
}

func WithCustomResourceDefinitionNames(names apiextensionsv1.CustomResourceDefinitionNames) CustomResourceDefinitionOption {
	return func(crd *apiextensionsv1.CustomResourceDefinition) {
		crd.Spec.Names = names
	}
}

func WithCustomResourceDefinitionDeletionTimestamp() CustomResourceDefinitionOption {
	return func(crd *apiextensionsv1.CustomResourceDefinition) {
		t := metav1.NewTime(time.Unix(1e9, 0))
		crd.ObjectMeta.SetDeletionTimestamp(&t)
	}
}
