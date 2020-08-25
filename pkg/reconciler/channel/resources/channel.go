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

package resources

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"knative.dev/pkg/kmeta"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "knative.dev/eventing/pkg/apis/messaging/v1"
)

// NewChannel returns an unstructured.Unstructured based on the ChannelTemplateSpec for a given Channel.
func NewChannel(c *v1.Channel) (*unstructured.Unstructured, error) {
	// Set the name of the resource we're creating as well as the namespace, etc.
	template := v1.ChannelTemplateSpecInternal{
		TypeMeta: metav1.TypeMeta{
			Kind:       c.Spec.ChannelTemplate.Kind,
			APIVersion: c.Spec.ChannelTemplate.APIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(c),
			},
			Name:      c.Name,
			Namespace: c.Namespace,
		},
		Spec: c.Spec.ChannelTemplate.Spec,
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
