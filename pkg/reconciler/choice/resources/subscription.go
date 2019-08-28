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
	"fmt"

	"knative.dev/pkg/kmeta"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
)

func ChoiceFilterSubscriptionName(choiceName string, caseNumber int) string {
	return fmt.Sprintf("%s-kn-choice-filter-%d", choiceName, caseNumber)
}

func ChoiceSubscriptionName(choiceName string, caseNumber int) string {
	return fmt.Sprintf("%s-kn-choice-%d", choiceName, caseNumber)
}

func NewFilterSubscription(caseNumber int, p *v1alpha1.Choice) *v1alpha1.Subscription {
	r := &v1alpha1.Subscription{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Subscription",
			APIVersion: "eventing.knative.dev/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: p.Namespace,
			Name:      ChoiceFilterSubscriptionName(p.Name, caseNumber),

			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(p),
			},
		},
		Spec: v1alpha1.SubscriptionSpec{
			Channel: corev1.ObjectReference{
				APIVersion: p.Spec.ChannelTemplate.APIVersion,
				Kind:       p.Spec.ChannelTemplate.Kind,
				Name:       ChoiceChannelName(p.Name),
			},
			Subscriber: p.Spec.Cases[caseNumber].Filter,
		},
	}
	r.Spec.Reply = &v1alpha1.ReplyStrategy{
		Channel: &corev1.ObjectReference{
			APIVersion: p.Spec.ChannelTemplate.APIVersion,
			Kind:       p.Spec.ChannelTemplate.Kind,
			Name:       ChoiceCaseChannelName(p.Name, caseNumber),
		}}
	return r
}

func NewSubscription(caseNumber int, p *v1alpha1.Choice) *v1alpha1.Subscription {
	r := &v1alpha1.Subscription{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Subscription",
			APIVersion: "eventing.knative.dev/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: p.Namespace,
			Name:      ChoiceSubscriptionName(p.Name, caseNumber),

			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(p),
			},
		},
		Spec: v1alpha1.SubscriptionSpec{
			Channel: corev1.ObjectReference{
				APIVersion: p.Spec.ChannelTemplate.APIVersion,
				Kind:       p.Spec.ChannelTemplate.Kind,
				Name:       ChoiceCaseChannelName(p.Name, caseNumber),
			},
			Subscriber: &p.Spec.Cases[caseNumber].Subscriber,
		},
	}

	if p.Spec.Cases[caseNumber].Reply != nil {
		r.Spec.Reply = &v1alpha1.ReplyStrategy{Channel: p.Spec.Cases[caseNumber].Reply}
	} else if p.Spec.Reply != nil {
		r.Spec.Reply = &v1alpha1.ReplyStrategy{Channel: p.Spec.Reply}
	}
	return r
}
