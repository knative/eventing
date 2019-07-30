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
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"knative.dev/pkg/kmeta"

	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	v1alpha1 "github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SequenceChannelName creates a name for the Channel fronting a specific step.
func SequenceChannelName(sequenceName string, step int) string {
	return fmt.Sprintf("%s-kn-sequence-%d", sequenceName, step)
}

// NewChannel returns an unstructured.Unstructured based on the ChannelTemplateSpec
// for a given sequence.
func NewChannel(name string, p *v1alpha1.Sequence) (*unstructured.Unstructured, error) {
	// Set the name of the resource we're creating as well as the namespace, etc.
	template := eventingduck.ChannelTemplateSpecInternal{
		TypeMeta: metav1.TypeMeta{
			Kind:       p.Spec.ChannelTemplate.Kind,
			APIVersion: p.Spec.ChannelTemplate.APIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(p),
			},
			Name:      name,
			Namespace: p.Namespace,
		},
		Spec: p.Spec.ChannelTemplate.Spec,
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
