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

	"knative.dev/pkg/kmeta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BrokerChannelName creates a name for the Channel for a Broker for a given
// Channel teyp
func BrokerChannelName(brokerName, channelType string) string {
	return fmt.Sprintf("%s-kn-%s", brokerName, channelType)
}

// NewChannel returns an unstructured.Unstructured based on the ChannelTemplateSpec
// for a given Broker.
func NewChannel(channelType string, b *v1alpha1.Broker, l map[string]string) (*unstructured.Unstructured, error) {
	// Set the name of the resource we're creating as well as the namespace, etc.
	template := v1alpha1.ChannelTemplateSpecInternal{
		metav1.TypeMeta{
			Kind:       b.Spec.ChannelTemplate.Kind,
			APIVersion: b.Spec.ChannelTemplate.APIVersion,
		},
		metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(b),
			},
			Name:      BrokerChannelName(b.Name, channelType),
			Namespace: b.Namespace,
			Labels:    l,
		},
		b.Spec.ChannelTemplate.Spec,
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
