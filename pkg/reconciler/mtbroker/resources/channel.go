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

package resources

import (
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"knative.dev/eventing/pkg/apis/eventing"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/pkg/kmeta"
)

// BrokerChannelName creates a name for the Channel for a Broker for a given
// Channel type.
func BrokerChannelName(brokerName, channelType string) string {
	return fmt.Sprintf("%s-kne-%s", brokerName, channelType)
}

// test
// NewChannel returns an unstructured.Unstructured based on the ChannelTemplateSpec
// for a given Broker.
func NewChannel(channelType string, owner kmeta.OwnerRefable, channelTemplate *messagingv1beta1.ChannelTemplateSpec, l map[string]string) (*unstructured.Unstructured, error) {
	// Set the name of the resource we're creating as well as the namespace, etc.
	template := messagingv1beta1.ChannelTemplateSpecInternal{
		TypeMeta: metav1.TypeMeta{
			Kind:       channelTemplate.Kind,
			APIVersion: channelTemplate.APIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(owner),
			},
			Name:        BrokerChannelName(owner.GetObjectMeta().GetName(), channelType),
			Namespace:   owner.GetObjectMeta().GetNamespace(),
			Labels:      l,
			Annotations: map[string]string{eventing.ScopeAnnotationKey: eventing.ScopeCluster},
		},
		Spec: channelTemplate.Spec,
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
