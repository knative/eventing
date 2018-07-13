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

package flow

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// CreateDynamicClient creates a dynamic client for the Group Version for the
// ObjectReference. It can only be used for that APIVersion / Group
func CreateDynamicClient(config *rest.Config, ref *corev1.ObjectReference) (*dynamic.Client, error) {
	// We need to tweak the configuration so that it points to the right
	// resources under the ThirdPartyResources that Istio uses.
	gvk := ref.GroupVersionKind()

	config.ContentConfig.GroupVersion = &schema.GroupVersion{
		Group:   gvk.Group,
		Version: gvk.Version,
	}
	config.APIPath = "apis"
	return dynamic.NewClient(config)
}

func CreateResourceInterface(config *rest.Config, ref *corev1.ObjectReference, namespace string) (dynamic.ResourceInterface, error) {
	c, err := CreateDynamicClient(config, ref)
	if err != nil {
		return nil, err
	}

	gvk := ref.GroupVersionKind()
	kind := gvk.Kind
	name := pluralizeKind(kind)

	resource := metav1.APIResource{
		Name:       name,
		Kind:       gvk.Kind,
		Namespaced: true,
	}
	r := c.Resource(&resource, namespace)
	if r == nil {
		return nil, fmt.Errorf("failed to create dynamic client resource")
	}
	return r, nil
}

// takes a kind and pluralizes it. This is super terrible, but I am
// not aware of a generic way to do this.
// I am not alone in thinking this and I haven't found a better solution:
// This seems relevant:
// https://github.com/kubernetes/kubernetes/issues/18622
func pluralizeKind(kind string) string {
	ret := strings.ToLower(kind)
	if strings.HasSuffix(ret, "s") {
		return fmt.Sprintf("%ses", ret)
	}
	return fmt.Sprintf("%ss", ret)
}
