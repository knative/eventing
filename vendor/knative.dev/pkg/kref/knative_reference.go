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

package kref

import (
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1lister "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// KReferenceResolver is an object that resolves the KReference.Group field
// Note: This API is EXPERIMENTAL and might break anytime. For more details: https://github.com/knative/eventing/issues/5086
type KReferenceResolver struct {
	crdLister apiextensionsv1lister.CustomResourceDefinitionLister
}

// NewKReferenceResolver creates a new KReferenceResolver from a crdLister
// Note: This API is EXPERIMENTAL and might break anytime. For more details: https://github.com/knative/eventing/issues/5086
func NewKReferenceResolver(crdLister apiextensionsv1lister.CustomResourceDefinitionLister) *KReferenceResolver {
	return &KReferenceResolver{crdLister: crdLister}
}

// ResolveGroup resolves the APIVersion of a KReference starting from the Group.
// In order to execute this method, you need RBAC to read the CRD of the Resource referred in this KReference.
// Note: This API is EXPERIMENTAL and might break anytime. For more details: https://github.com/knative/eventing/issues/5086
func (resolver *KReferenceResolver) ResolveGroup(kr *duckv1.KReference) (*duckv1.KReference, error) {
	if kr.Group == "" {
		// Nothing to do here
		return kr, nil
	}

	kr = kr.DeepCopy()

	actualGvk := schema.GroupVersionKind{Group: kr.Group, Kind: kr.Kind}
	pluralGvk, _ := meta.UnsafeGuessKindToResource(actualGvk)
	crd, err := resolver.crdLister.Get(pluralGvk.GroupResource().String())
	if err != nil {
		return nil, err
	}

	actualGvk.Version, err = findCRDStorageVersion(crd)
	if err != nil {
		return nil, err
	}

	kr.APIVersion, kr.Kind = actualGvk.ToAPIVersionAndKind()

	return kr, nil
}

// This function runs under the assumption that there must be exactly one "storage" version
func findCRDStorageVersion(crd *apiextensionsv1.CustomResourceDefinition) (string, error) {
	for _, version := range crd.Spec.Versions {
		if version.Storage {
			return version.Name, nil
		}
	}
	return "", fmt.Errorf("this CRD %s doesn't have a storage version! Kubernetes, you're drunk, go home", crd.Name)
}
