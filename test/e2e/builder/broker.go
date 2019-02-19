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

package builder

import (
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type BrokerBuilder struct {
	*eventingv1alpha1.Broker
}

func Broker(name, namespace string) *BrokerBuilder {
	broker := &eventingv1alpha1.Broker{
		TypeMeta: metav1.TypeMeta{
			APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			Kind:       "Broker",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: eventingv1alpha1.BrokerSpec{},
	}

	return &BrokerBuilder{
		Broker: broker,
	}
}

func (b *BrokerBuilder) Build() runtime.Object {
	return b.Broker.DeepCopy()
}