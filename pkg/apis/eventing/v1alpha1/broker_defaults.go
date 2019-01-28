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

package v1alpha1

import v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	brokerLabel = "eventing.knative.dev/broker"
)

func (b *Broker) SetDefaults() {
	b.Spec.SetDefaults(b.Name)
}

func (bs *BrokerSpec) SetDefaults(brokerName string) {
	if bs.Selector == nil {
		bs.Selector = defaultBrokerSpecSelector(brokerName)
	}
	if bs.ChannelTemplate == nil {
		bs.ChannelTemplate = defaultBrokerSpecChannelTemplate(brokerName)
	}
	if len(bs.SubscribableResources) == 0 {
		bs.SubscribableResources = defaultBrokerSpecSubscribableResources()
	}
}

func defaultBrokerLabels(brokerName string) map[string]string {
	return map[string]string{
		brokerLabel: brokerName,
	}
}

func defaultBrokerSpecSelector(brokerName string) *v1.LabelSelector {
	return &v1.LabelSelector{
		MatchLabels: defaultBrokerLabels(brokerName),
	}
}

func defaultBrokerSpecChannelTemplate(brokerName string) *ChannelTemplateSpec {
	return &ChannelTemplateSpec{
		Metadata: v1.ObjectMeta{
			Labels: defaultBrokerLabels(brokerName),
		},
		// Spec is left blank so that the created Channel defaulter will default the provisioner
		// and arguments when the Channel is created.
	}
}

func defaultBrokerSpecSubscribableResources() []v1.GroupVersionKind {
	return []v1.GroupVersionKind{
		{
			Group:   "eventing.knative.dev",
			Version: "v1alpha1",
			Kind:    "Channel",
		},
	}
}
