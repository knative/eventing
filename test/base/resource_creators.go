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

// This file contains functions which create actual resources given the meta resource.

package base

import (
	"encoding/json"

	"github.com/knative/eventing/test/base/resources"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// CreateGenericChannelObject create a generic channel object with the dynamic client and channel's meta data.
func CreateGenericChannelObject(
	dynamicClient dynamic.Interface,
	obj *resources.MetaResource,
) ( /*plural*/ schema.GroupVersionResource, error) {
	// get the resource's namespace and gvr
	gvr, _ := meta.UnsafeGuessKindToResource(obj.GroupVersionKind())
	newChannel, err := newChannel(obj)
	if err != nil {
		return gvr, err
	}

	_, err = dynamicClient.Resource(gvr).Namespace(obj.Namespace).Create(newChannel, metav1.CreateOptions{})
	return gvr, err
}

// newChannel returns an unstructured.Unstructured based on the ChannelTemplateSpec for a given sequence.
func newChannel(obj *resources.MetaResource) (*unstructured.Unstructured, error) {
	// Set the name of the resource we're creating as well as the namespace, etc.
	template := eventingduck.ChannelTemplateSpecInternal{
		TypeMeta: metav1.TypeMeta{
			Kind:       obj.Kind,
			APIVersion: obj.APIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		},
		Spec: eventingduck.ChannelTemplateSpec{
			TypeMeta: obj.TypeMeta,
		}.Spec,
	}
	raw, err := json.Marshal(template)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{}
	err = json.Unmarshal(raw, u)
	if err != nil {
		return nil, err
	}
	return u, nil
}
