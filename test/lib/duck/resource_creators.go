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

package duck

import (
	"context"
	"encoding/json"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"knative.dev/eventing/test/lib/resources"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
)

// CreateGenericChannelObject create a generic channel object with the dynamic client and channel's meta data.
func CreateGenericChannelObject(
	dynamicClient dynamic.Interface,
	obj *resources.MetaResource,
) ( /*plural*/ schema.GroupVersionResource, error) {
	// get the resource's gvr
	gvr, _ := meta.UnsafeGuessKindToResource(obj.GroupVersionKind())
	newChannel, err := newChannel(obj)
	if err != nil {
		return gvr, err
	}

	channelResourceInterface := dynamicClient.Resource(gvr).Namespace(obj.Namespace)
	_, err = channelResourceInterface.Create(context.Background(), newChannel, metav1.CreateOptions{})
	return gvr, err
}

// newChannel returns an unstructured.Unstructured based on the ChannelTemplateSpec for a given meta resource.
func newChannel(obj *resources.MetaResource) (*unstructured.Unstructured, error) {
	// Set the name of the resource we're creating as well as the namespace, etc.
	template := messagingv1beta1.ChannelTemplateSpecInternal{
		TypeMeta: metav1.TypeMeta{
			Kind:       obj.Kind,
			APIVersion: obj.APIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		},
		Spec: messagingv1beta1.ChannelTemplateSpec{
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
