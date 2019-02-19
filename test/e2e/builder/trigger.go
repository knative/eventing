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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type TriggerBuilder struct {
	*eventingv1alpha1.Trigger
}

func Trigger(name, namespace string) *TriggerBuilder {
	trigger := &eventingv1alpha1.Trigger{
		TypeMeta: metav1.TypeMeta{
			APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			Kind:       "Trigger",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: eventingv1alpha1.TriggerSpec{},
	}

	return &TriggerBuilder{
		Trigger: trigger,
	}
}

func (b *TriggerBuilder) Build() runtime.Object {
	return b.Trigger.DeepCopy()
}

func (b *TriggerBuilder) Type(eventType string) *TriggerBuilder {
	b.Trigger.Spec.Type = eventType
	return b
}

func (b *TriggerBuilder) Broker(brokerName string) *TriggerBuilder {
	b.Trigger.Spec.Broker = brokerName
	return b
}

func (b *TriggerBuilder) Subscriber(ref *corev1.ObjectReference) *TriggerBuilder {
	b.Trigger.Spec.Subscriber.Ref = ref
	return b
}

func (b *TriggerBuilder) SubscriberSvc(svcName string) *TriggerBuilder {
	b.Trigger.Spec.Subscriber.Ref = &corev1.ObjectReference{
		APIVersion: corev1.SchemeGroupVersion.String(),
		Kind: "Service",
		Name: svcName,
		Namespace: b.Trigger.GetNamespace(),
	}
	return b
}