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

func ParallelFilterSubscriptionName(parallelName string, branchNumber int) string {
	return fmt.Sprintf("%s-kn-parallel-filter-%d", parallelName, branchNumber)
}

func ParallelSubscriptionName(parallelName string, branchNumber int) string {
	return fmt.Sprintf("%s-kn-parallel-%d", parallelName, branchNumber)
}

func NewFilterSubscription(branchNumber int, p *v1alpha1.Parallel) *v1alpha1.Subscription {
	var subscriberSpec *v1alpha1.SubscriberSpec
	if p.Spec.Branches[branchNumber].Filter != nil {
		subscriberSpec = &v1alpha1.SubscriberSpec{Ref: p.Spec.Branches[branchNumber].Filter.GetRef()}
		if p.Spec.Branches[branchNumber].Filter.URI != nil {
			uri := p.Spec.Branches[branchNumber].Filter.URI.String()
			subscriberSpec.URI = &uri
		}
	}
	r := &v1alpha1.Subscription{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Subscription",
			APIVersion: "eventing.knative.dev/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: p.Namespace,
			Name:      ParallelFilterSubscriptionName(p.Name, branchNumber),

			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(p),
			},
		},
		Spec: v1alpha1.SubscriptionSpec{
			Channel: corev1.ObjectReference{
				APIVersion: p.Spec.ChannelTemplate.APIVersion,
				Kind:       p.Spec.ChannelTemplate.Kind,
				Name:       ParallelChannelName(p.Name),
			},
			Subscriber: subscriberSpec,
		},
	}
	r.Spec.Reply = &v1alpha1.ReplyStrategy{
		Channel: &corev1.ObjectReference{
			APIVersion: p.Spec.ChannelTemplate.APIVersion,
			Kind:       p.Spec.ChannelTemplate.Kind,
			Name:       ParallelBranchChannelName(p.Name, branchNumber),
		}}
	return r
}

func NewSubscription(branchNumber int, p *v1alpha1.Parallel) *v1alpha1.Subscription {
	subscriberSpec := &v1alpha1.SubscriberSpec{Ref: p.Spec.Branches[branchNumber].Subscriber.GetRef()}
	if p.Spec.Branches[branchNumber].Subscriber.URI != nil {
		uri := p.Spec.Branches[branchNumber].Subscriber.URI.String()
		subscriberSpec.URI = &uri
	}
	r := &v1alpha1.Subscription{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Subscription",
			APIVersion: "eventing.knative.dev/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: p.Namespace,
			Name:      ParallelSubscriptionName(p.Name, branchNumber),

			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(p),
			},
		},
		Spec: v1alpha1.SubscriptionSpec{
			Channel: corev1.ObjectReference{
				APIVersion: p.Spec.ChannelTemplate.APIVersion,
				Kind:       p.Spec.ChannelTemplate.Kind,
				Name:       ParallelBranchChannelName(p.Name, branchNumber),
			},
			Subscriber: subscriberSpec,
		},
	}

	if p.Spec.Branches[branchNumber].Reply != nil {
		r.Spec.Reply = &v1alpha1.ReplyStrategy{Channel: p.Spec.Branches[branchNumber].Reply.GetRef()}
	} else if p.Spec.Reply != nil {
		r.Spec.Reply = &v1alpha1.ReplyStrategy{Channel: p.Spec.Reply.GetRef()}
	}
	return r
}
